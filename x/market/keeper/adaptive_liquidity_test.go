package keeper

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	core "github.com/classic-terra/core/v4/types"
	"github.com/classic-terra/core/v4/x/market/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestComputeAdaptiveLiquidityParamsProposalDefault(t *testing.T) {
	// The proposal defines F=7%. With 600M LUNC worth 24,000 SDR and a
	// 6.5T post-burn supply, the adaptive base pool is 5,460 SDR.
	adaptive, err := ComputeAdaptiveLiquidityParams(
		sdkmath.NewInt(600_000_000_000_000),
		sdkmath.NewInt(6_500_000_000_000_000_000),
		sdkmath.LegacyMustNewDecFromStr("0.00004"),
		adaptiveBurstFactor,
	)
	require.NoError(t, err)
	require.Equal(t, uint64(93_600), adaptive.PoolRecoveryPeriod)
	require.Equal(t, sdkmath.LegacyNewDec(24_000_000_000), adaptive.LunaPoolValueSDR)
	require.Equal(t, sdkmath.LegacyNewDec(5_460_000_000), adaptive.BasePool)
}

func TestComputeAdaptiveLiquidityParamsShrinksWithSupply(t *testing.T) {
	pool := sdkmath.NewInt(600_000_000_000_000)
	rate := sdkmath.LegacyMustNewDecFromStr("0.00004")
	factor := adaptiveBurstFactor

	highSupply, err := ComputeAdaptiveLiquidityParams(
		pool,
		sdkmath.NewInt(6_500_000_000_000_000_000),
		rate,
		factor,
	)
	require.NoError(t, err)

	oneTrillionSupply, err := ComputeAdaptiveLiquidityParams(
		pool,
		sdkmath.NewInt(1_000_000_000_000_000_000),
		rate,
		factor,
	)
	require.NoError(t, err)
	require.Equal(t, uint64(core.BlocksPerDay), oneTrillionSupply.PoolRecoveryPeriod)
	require.Equal(t, sdkmath.LegacyNewDec(840_000_000), oneTrillionSupply.BasePool)
	require.True(t, oneTrillionSupply.BasePool.LT(highSupply.BasePool))
}

func TestAdaptiveBurstFactorIsSeparateFromHardDailyCap(t *testing.T) {
	input := CreateTestInput(t)
	require.Equal(t, sdkmath.LegacyMustNewDecFromStr("0.07"), adaptiveBurstFactor)
	require.Equal(t, sdkmath.LegacyMustNewDecFromStr("0.10"), input.MarketKeeper.DailyCapFactor(input.Ctx))
}

func TestComputeAdaptiveLiquidityParamsClamps(t *testing.T) {
	t.Run("supply value cap", func(t *testing.T) {
		adaptive, err := ComputeAdaptiveLiquidityParams(
			sdkmath.NewInt(1_000_000_000_000_000_000),
			sdkmath.NewInt(1_000_000_000_000_000_000),
			sdkmath.LegacyMustNewDecFromStr("0.001"),
			sdkmath.LegacyMustNewDecFromStr("0.10"),
		)
		require.NoError(t, err)
		require.Equal(t, sdkmath.LegacyNewDec(100_000_000_000), adaptive.BasePool)
	})

	t.Run("absolute cap", func(t *testing.T) {
		adaptive, err := ComputeAdaptiveLiquidityParams(
			sdkmath.NewInt(6_000_000_000_000_000_000),
			sdkmath.NewInt(6_500_000_000_000_000_000),
			sdkmath.LegacyOneDec(),
			sdkmath.LegacyMustNewDecFromStr("0.10"),
		)
		require.NoError(t, err)
		require.Equal(t, sdkmath.LegacyNewDec(5_000_000_000_000), adaptive.BasePool)
	})
}

func TestComputeAdaptiveLiquidityParamsRejectsInvalidInputs(t *testing.T) {
	validPool := sdkmath.OneInt()
	validSupply := sdkmath.NewInt(1_000_000_000_000_000_000)
	validRate := sdkmath.LegacyOneDec()
	validFactor := sdkmath.LegacyMustNewDecFromStr("0.10")

	_, err := ComputeAdaptiveLiquidityParams(sdkmath.ZeroInt(), validSupply, validRate, validFactor)
	require.Error(t, err)
	_, err = ComputeAdaptiveLiquidityParams(validPool, sdkmath.ZeroInt(), validRate, validFactor)
	require.Error(t, err)
	_, err = ComputeAdaptiveLiquidityParams(validPool, validSupply, sdkmath.LegacyZeroDec(), validFactor)
	require.Error(t, err)
	_, err = ComputeAdaptiveLiquidityParams(validPool, validSupply, validRate, sdkmath.LegacyZeroDec())
	require.Error(t, err)
}

func TestEpochDefersWhenAdaptiveOracleIsUnavailable(t *testing.T) {
	input := CreateTestInput(t)
	marketRemainder := sdk.NewCoins(sdk.NewInt64Coin(core.MicroUSDDenom, 1_000))
	accumulator := sdk.NewCoins(
		sdk.NewInt64Coin(core.MicroLunaDenom, 2_000),
		sdk.NewInt64Coin(core.MicroUSDDenom, 3_000),
	)

	require.NoError(t, FundModuleAccount(input, types.ModuleName, marketRemainder))
	require.NoError(t, input.BankKeeper.MintCoins(input.Ctx, faucetAccountName, accumulator))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(
		input.Ctx, faucetAccountName, types.AccumulatorModuleName, accumulator,
	))

	input.Ctx = input.Ctx.WithBlockHeight(1)
	input.MarketKeeper.ProcessEpochIfDue(input.Ctx)

	marketAddr := input.AccountKeeper.GetModuleAddress(types.ModuleName)
	accumulatorAddr := input.AccountKeeper.GetModuleAddress(types.AccumulatorModuleName)
	require.Equal(t, marketRemainder, input.BankKeeper.GetAllBalances(input.Ctx, marketAddr))
	require.Equal(t, accumulator, input.BankKeeper.GetAllBalances(input.Ctx, accumulatorAddr))
	require.Zero(t, input.MarketKeeper.GetLastEpochHeight(input.Ctx))

	setFreshAdaptiveOracle(input)
	input.MarketKeeper.ProcessEpochIfDue(input.Ctx)
	require.Equal(t, accumulator, input.BankKeeper.GetAllBalances(input.Ctx, marketAddr))
	require.True(t, input.BankKeeper.GetAllBalances(input.Ctx, accumulatorAddr).Empty())
	require.Equal(t, int64(1), input.MarketKeeper.GetLastEpochHeight(input.Ctx))
}
