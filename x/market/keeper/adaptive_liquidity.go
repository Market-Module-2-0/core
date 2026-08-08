package keeper

import (
	"fmt"

	"cosmossdk.io/math"
	core "github.com/classic-terra/core/v4/types"
	"github.com/classic-terra/core/v4/x/market/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	oneTrillionMicroLuna = math.NewInt(1_000_000_000_000).MulRaw(core.MicroUnit)
	// Section 4.1 defines F=0.07. This adaptive target intentionally remains
	// separate from the direct 10% per-denom safety cap.
	adaptiveBurstFactor  = math.LegacyMustNewDecFromStr("0.07")
	supplyValueCapFactor = math.LegacyMustNewDecFromStr("0.00010")
	absoluteBasePoolCap  = math.LegacyNewDec(5_000_000 * core.MicroUnit)
	maximumInt64         = math.NewInt(9_223_372_036_854_775_807)
)

// AdaptiveLiquidityParams contains the virtual-pool settings derived for a new
// physical-liquidity epoch. All SDR amounts use the micro-SDR unit used by the
// legacy Market module.
type AdaptiveLiquidityParams struct {
	BasePool            math.LegacyDec
	PoolRecoveryPeriod  uint64
	LunaPoolValueSDR    math.LegacyDec
	LunaSupplyValueSDR  math.LegacyDec
	LunaSupplyAfterBurn math.Int
}

// ComputeAdaptiveLiquidityParams implements the proposal's epoch formula:
//
//	PRP = max(blocks/day, blocks/day * LUNC supply / 1T LUNC)
//	raw base_pool = (LUNC pool value in SDR * daily factor * PRP) / (2 * blocks/day)
//	base_pool = min(raw base_pool, 0.00010 * supply value in SDR, 5M SDR)
//
// The direct per-denom daily cap remains the hard enforcement layer. These
// parameters control the legacy virtual-pool spread and recovery curve.
func ComputeAdaptiveLiquidityParams(
	lunaPoolBalance math.Int,
	lunaSupplyAfterBurn math.Int,
	sdrPerLuna math.LegacyDec,
	burstFactor math.LegacyDec,
) (AdaptiveLiquidityParams, error) {
	if !lunaPoolBalance.IsPositive() {
		return AdaptiveLiquidityParams{}, fmt.Errorf("LUNC epoch pool must be positive")
	}
	if !lunaSupplyAfterBurn.IsPositive() {
		return AdaptiveLiquidityParams{}, fmt.Errorf("post-burn LUNC supply must be positive")
	}
	if !sdrPerLuna.IsPositive() {
		return AdaptiveLiquidityParams{}, fmt.Errorf("LUNC/SDR oracle rate must be positive")
	}
	if !burstFactor.IsPositive() || burstFactor.GT(math.LegacyOneDec()) {
		return AdaptiveLiquidityParams{}, fmt.Errorf("adaptive burst factor must be in (0,1]")
	}

	blocksPerDay := math.LegacyNewDec(int64(core.BlocksPerDay))
	supplyRatio := math.LegacyNewDecFromInt(lunaSupplyAfterBurn).
		QuoInt(oneTrillionMicroLuna)
	prpDec := blocksPerDay.Mul(supplyRatio)
	if prpDec.LT(blocksPerDay) {
		prpDec = blocksPerDay
	}

	prpInt := prpDec.Ceil().TruncateInt()
	if !prpInt.IsUint64() || prpInt.GT(maximumInt64) {
		return AdaptiveLiquidityParams{}, fmt.Errorf("calculated pool recovery period exceeds int64")
	}
	prp := prpInt.Uint64()

	lunaPoolValueSDR := math.LegacyNewDecFromInt(lunaPoolBalance).Mul(sdrPerLuna)
	lunaSupplyValueSDR := math.LegacyNewDecFromInt(lunaSupplyAfterBurn).Mul(sdrPerLuna)
	desiredDailyCap := lunaPoolValueSDR.Mul(burstFactor)
	rawBasePool := desiredDailyCap.MulInt(prpInt).
		QuoInt64(2 * int64(core.BlocksPerDay))
	supplyCap := lunaSupplyValueSDR.Mul(supplyValueCapFactor)

	basePool := rawBasePool
	if supplyCap.LT(basePool) {
		basePool = supplyCap
	}
	if absoluteBasePoolCap.LT(basePool) {
		basePool = absoluteBasePoolCap
	}

	return AdaptiveLiquidityParams{
		BasePool:            basePool,
		PoolRecoveryPeriod:  prp,
		LunaPoolValueSDR:    lunaPoolValueSDR,
		LunaSupplyValueSDR:  lunaSupplyValueSDR,
		LunaSupplyAfterBurn: lunaSupplyAfterBurn,
	}, nil
}

// prepareAdaptiveLiquidityParams computes the next epoch settings without
// mutating state. A missing LUNC side returns nil so the physical pool can still
// rotate, while first activation remains blocked by its two-sided-liquidity rule.
func (k Keeper) prepareAdaptiveLiquidityParams(
	ctx sdk.Context,
	marketBalances sdk.Coins,
	accumulatorBalances sdk.Coins,
) (*AdaptiveLiquidityParams, error) {
	lunaPoolBalance := accumulatorBalances.AmountOf(core.MicroLunaDenom)
	if !lunaPoolBalance.IsPositive() {
		return nil, nil
	}

	sdrPerLuna, err := k.OracleKeeper.GetLunaExchangeRate(ctx, core.MicroSDRDenom)
	if err != nil {
		return nil, fmt.Errorf("adaptive liquidity requires a positive LUNC/SDR oracle rate: %w", err)
	}
	if !sdrPerLuna.IsPositive() {
		return nil, fmt.Errorf("adaptive liquidity requires a positive LUNC/SDR oracle rate")
	}

	lastTallyTime := k.GetLastOracleTallyTime(ctx)
	currentTime := ctx.BlockTime().Unix()
	if lastTallyTime <= 0 || currentTime < lastTallyTime ||
		currentTime-lastTallyTime > int64(k.MaxOracleAgeSeconds(ctx)) {
		return nil, types.ErrOraclePriceStale
	}

	lunaSupply := k.BankKeeper.GetSupply(ctx, core.MicroLunaDenom).Amount
	lunaToBurn := marketBalances.AmountOf(core.MicroLunaDenom)
	if lunaToBurn.GT(lunaSupply) {
		return nil, fmt.Errorf("market LUNC balance exceeds total supply")
	}
	lunaSupplyAfterBurn := lunaSupply.Sub(lunaToBurn)

	params, err := ComputeAdaptiveLiquidityParams(
		lunaPoolBalance,
		lunaSupplyAfterBurn,
		sdrPerLuna,
		adaptiveBurstFactor,
	)
	if err != nil {
		return nil, err
	}

	return &params, nil
}

func (k Keeper) applyAdaptiveLiquidityParams(ctx sdk.Context, adaptive *AdaptiveLiquidityParams) {
	if adaptive == nil {
		return
	}

	params := k.GetParams(ctx)
	params.BasePool = adaptive.BasePool
	params.PoolRecoveryPeriod = adaptive.PoolRecoveryPeriod
	k.SetParams(ctx, params)
	// A newly sized virtual pool starts the epoch at equilibrium. Carrying the
	// previous delta into a smaller base pool could make the curve invalid.
	k.SetTerraPoolDelta(ctx, math.LegacyZeroDec())

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventAdaptiveLiquidity,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(types.AttributeKeyBasePool, adaptive.BasePool.String()),
			sdk.NewAttribute(types.AttributeKeyPoolRecoveryPeriod, fmt.Sprintf("%d", adaptive.PoolRecoveryPeriod)),
			sdk.NewAttribute(types.AttributeKeyLunaPoolValueSDR, adaptive.LunaPoolValueSDR.String()),
			sdk.NewAttribute(types.AttributeKeyLunaSupply, adaptive.LunaSupplyAfterBurn.String()),
		),
	)
}
