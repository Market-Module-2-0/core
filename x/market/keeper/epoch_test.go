package keeper

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	core "github.com/classic-terra/core/v4/types"
	"github.com/classic-terra/core/v4/x/market/types"
	oracletypes "github.com/classic-terra/core/v4/x/oracle/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func setFreshAdaptiveOracle(input TestInput) {
	input.OracleKeeper.SetLunaExchangeRate(input.Ctx, core.MicroSDRDenom, mm2LUNCSDR)
	input.MarketKeeper.SetLastOracleTallyTime(input.Ctx, input.Ctx.BlockTime().Unix())
}

func TestEpoch_BurnAndRefill(t *testing.T) {
	input := CreateTestInput(t)

	marketAddr := input.AccountKeeper.GetModuleAddress(types.ModuleName)
	accumAddr := input.AccountKeeper.GetModuleAddress(types.AccumulatorModuleName)

	// Seed balances: market has 1_000_000 uusd; accumulator has 5_000_000 uusd
	preMarket := sdk.NewCoins(sdk.NewInt64Coin(core.MicroUSDDenom, 1_000_000))
	preAccum := sdk.NewCoins(sdk.NewInt64Coin(core.MicroUSDDenom, 5_000_000))

	require.NoError(t, input.BankKeeper.MintCoins(input.Ctx, faucetAccountName, preMarket))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(input.Ctx, faucetAccountName, types.ModuleName, preMarket))
	require.Equal(t, preMarket, input.BankKeeper.GetAllBalances(input.Ctx, marketAddr))

	require.NoError(t, input.BankKeeper.MintCoins(input.Ctx, faucetAccountName, preAccum))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(input.Ctx, faucetAccountName, types.AccumulatorModuleName, preAccum))
	require.Equal(t, preAccum, input.BankKeeper.GetAllBalances(input.Ctx, accumAddr))

	// Set non-zero height and trigger epoch processing: since last epoch is 0, it should process now
	input.Ctx = input.Ctx.WithBlockHeight(1)
	input.MarketKeeper.ProcessEpochIfDue(input.Ctx)
	input.MarketKeeper.ReplenishPools(input.Ctx)

	// Market balance should equal pre-accumulator (burned its own pre balance then refilled)
	require.Equal(t, preAccum, input.BankKeeper.GetAllBalances(input.Ctx, marketAddr))
	// Accumulator should be empty
	require.True(t, input.BankKeeper.GetAllBalances(input.Ctx, accumAddr).Empty())
}

func TestEpoch_NoProcessBeforeEpoch(t *testing.T) {
	input := CreateTestInput(t)

	marketAddr := input.AccountKeeper.GetModuleAddress(types.ModuleName)
	accumAddr := input.AccountKeeper.GetModuleAddress(types.AccumulatorModuleName)

	// First processing to set last epoch height at height 1
	initial := sdk.NewCoins(sdk.NewInt64Coin(core.MicroUSDDenom, 100_000))
	require.NoError(t, input.BankKeeper.MintCoins(input.Ctx, faucetAccountName, initial))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(input.Ctx, faucetAccountName, types.AccumulatorModuleName, initial))
	input.Ctx = input.Ctx.WithBlockHeight(1)
	input.MarketKeeper.ProcessEpochIfDue(input.Ctx)
	input.MarketKeeper.ReplenishPools(input.Ctx)
	require.Equal(t, initial, input.BankKeeper.GetAllBalances(input.Ctx, marketAddr))
	require.True(t, input.BankKeeper.GetAllBalances(input.Ctx, accumAddr).Empty())

	// Mint new amounts to both accounts
	moreMarket := sdk.NewCoins(sdk.NewInt64Coin(core.MicroUSDDenom, 222_222))
	moreAccum := sdk.NewCoins(sdk.NewInt64Coin(core.MicroUSDDenom, 333_333))
	require.NoError(t, input.BankKeeper.MintCoins(input.Ctx, faucetAccountName, moreMarket))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(input.Ctx, faucetAccountName, types.ModuleName, moreMarket))
	require.NoError(t, input.BankKeeper.MintCoins(input.Ctx, faucetAccountName, moreAccum))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(input.Ctx, faucetAccountName, types.AccumulatorModuleName, moreAccum))

	// Advance height but not enough for epoch: should NOT process epoch
	input.Ctx = input.Ctx.WithBlockHeight(2)
	input.MarketKeeper.ProcessEpochIfDue(input.Ctx)
	input.MarketKeeper.ReplenishPools(input.Ctx)

	// Balances remain unchanged
	expectedMarket := initial.Add(moreMarket...)
	require.Equal(t, expectedMarket, input.BankKeeper.GetAllBalances(input.Ctx, marketAddr))
	require.Equal(t, moreAccum, input.BankKeeper.GetAllBalances(input.Ctx, accumAddr))
}

func TestInitialActivationAfterFullEpoch(t *testing.T) {
	input := CreateTestInput(t)
	params := input.MarketKeeper.GetParams(input.Ctx)
	params.EpochLengthBlocks = 10
	input.MarketKeeper.SetParams(input.Ctx, params)

	store := input.Ctx.KVStore(input.MarketKeeper.storeKey)
	store.Delete(types.MarketEnabledKey)
	store.Delete(types.InitialActivationPendingKey)
	store.Delete(types.EpochLastHeightKey)

	input.Ctx = input.Ctx.WithBlockHeight(100)
	input.MarketKeeper.InitializeMarketForUpgrade(input.Ctx)
	require.False(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))
	require.True(t, input.MarketKeeper.IsInitialActivationPending(input.Ctx))
	require.Equal(t, int64(100), input.MarketKeeper.GetLastEpochHeight(input.Ctx))

	server := NewMsgServerImpl(input.MarketKeeper)
	firstSwap := types.NewMsgSwap(
		Addrs[0],
		sdk.NewInt64Coin(core.MicroLunaDenom, 100_000),
		core.MicroUSDDenom,
	)
	_, err := server.Swap(sdk.WrapSDKContext(input.Ctx), firstSwap)
	require.ErrorIs(t, err, types.ErrMarketDisabled)

	input.Ctx = input.Ctx.WithBlockHeight(105)
	input.MarketKeeper.InitializeMarketForUpgrade(input.Ctx)
	require.Equal(t, int64(100), input.MarketKeeper.GetLastEpochHeight(input.Ctx))

	liquidity := sdk.NewCoins(
		sdk.NewInt64Coin(core.MicroLunaDenom, 5_000_000),
		sdk.NewInt64Coin(core.MicroUSDDenom, 25_000),
	)
	setFreshAdaptiveOracle(input)
	require.NoError(t, input.BankKeeper.MintCoins(input.Ctx, faucetAccountName, liquidity))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(
		input.Ctx, faucetAccountName, types.AccumulatorModuleName, liquidity,
	))
	expectedAdaptive, err := ComputeAdaptiveLiquidityParams(
		liquidity.AmountOf(core.MicroLunaDenom),
		input.BankKeeper.GetSupply(input.Ctx, core.MicroLunaDenom).Amount,
		mm2LUNCSDR,
		adaptiveBurstFactor,
	)
	require.NoError(t, err)

	input.Ctx = input.Ctx.WithBlockHeight(109)
	input.MarketKeeper.ProcessEpochIfDue(input.Ctx)
	require.False(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))
	require.Equal(t, liquidity, input.BankKeeper.GetAllBalances(
		input.Ctx, input.AccountKeeper.GetModuleAddress(types.AccumulatorModuleName),
	))

	input.Ctx = input.Ctx.WithBlockHeight(110).WithEventManager(sdk.NewEventManager())
	input.MarketKeeper.ProcessEpochIfDue(input.Ctx)
	require.True(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))
	require.False(t, input.MarketKeeper.IsInitialActivationPending(input.Ctx))
	require.Equal(t, liquidity, input.BankKeeper.GetAllBalances(
		input.Ctx, input.AccountKeeper.GetModuleAddress(types.ModuleName),
	))

	activationEventFound := false
	adaptiveEventFound := false
	for _, event := range input.Ctx.EventManager().Events() {
		if event.Type == types.EventActivation {
			activationEventFound = true
		}
		if event.Type == types.EventAdaptiveLiquidity {
			adaptiveEventFound = true
		}
	}
	require.True(t, activationEventFound)
	require.True(t, adaptiveEventFound)
	require.Equal(t, expectedAdaptive.BasePool, input.MarketKeeper.BasePool(input.Ctx))
	require.Equal(t, expectedAdaptive.PoolRecoveryPeriod, input.MarketKeeper.PoolRecoveryPeriod(input.Ctx))

	input.OracleKeeper.SetLunaExchangeRate(input.Ctx, core.MicroUSDDenom, mm2LUNCUSD)
	input.OracleKeeper.SetLunaExchangeRate(input.Ctx, oracletypes.MetaUSDDenom, mm2USTCUSD)
	input.OracleKeeper.SetLunaExchangeRate(input.Ctx, core.MicroSDRDenom, mm2LUNCSDR)
	input.MarketKeeper.SetLastOracleTallyTime(input.Ctx, input.Ctx.BlockTime().Unix())
	SeedCompleteTWAP(&input, map[string]sdkmath.LegacyDec{
		core.MicroUSDDenom:       mm2LUNCUSD,
		oracletypes.MetaUSDDenom: mm2USTCUSD,
	})

	response, err := server.Swap(sdk.WrapSDKContext(input.Ctx), firstSwap)
	require.NoError(t, err)
	require.True(t, response.SwapCoin.IsPositive())
}

func TestInitialActivationWaitsForBothPools(t *testing.T) {
	input := CreateTestInput(t)
	params := input.MarketKeeper.GetParams(input.Ctx)
	params.EpochLengthBlocks = 10
	input.MarketKeeper.SetParams(input.Ctx, params)

	store := input.Ctx.KVStore(input.MarketKeeper.storeKey)
	store.Delete(types.MarketEnabledKey)
	store.Delete(types.InitialActivationPendingKey)
	store.Delete(types.EpochLastHeightKey)

	input.Ctx = input.Ctx.WithBlockHeight(100)
	input.MarketKeeper.InitializeMarketForUpgrade(input.Ctx)

	ustcOnly := sdk.NewCoins(sdk.NewInt64Coin(core.MicroUSDDenom, 25_000))
	require.NoError(t, input.BankKeeper.MintCoins(input.Ctx, faucetAccountName, ustcOnly))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(
		input.Ctx, faucetAccountName, types.AccumulatorModuleName, ustcOnly,
	))

	input.Ctx = input.Ctx.WithBlockHeight(110)
	input.MarketKeeper.ProcessEpochIfDue(input.Ctx)
	require.False(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))
	require.True(t, input.MarketKeeper.IsInitialActivationPending(input.Ctx))
}
