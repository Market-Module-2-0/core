package keeper

import (
	"testing"

	core "github.com/classic-terra/core/v4/types"
	markettypes "github.com/classic-terra/core/v4/x/market/types"
	oracletypes "github.com/classic-terra/core/v4/x/oracle/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func ustcVotePowers(ustPower, usdPower, totalPower int64) []oracletypes.DenomVotePower {
	return []oracletypes.DenomVotePower{
		{Denom: oracletypes.MetaUSDDenom, VotePower: ustPower, TotalPower: totalPower},
		{Denom: core.MicroUSDDenom, VotePower: usdPower, TotalPower: totalPower},
	}
}

func TestOracleQuorumHaltsAfter25ConsecutiveBlocks(t *testing.T) {
	input := CreateTestInput(t)
	input.MarketKeeper.SetMarketAssets(defaultMarketAssets())
	input.Ctx = input.Ctx.WithEventManager(sdk.NewEventManager())

	for tally := uint64(1); tally <= 5; tally++ {
		input.Ctx = input.Ctx.WithBlockHeight(int64(tally * 5))
		input.MarketKeeper.AfterOracleTally(input.Ctx, 5, ustcVotePowers(49, 80, 100))

		require.Equal(t, tally*5, input.MarketKeeper.GetOracleMissedBlocks(
			input.Ctx, oracletypes.MetaUSDDenom,
		))
		require.Zero(t, input.MarketKeeper.GetOracleMissedBlocks(input.Ctx, core.MicroUSDDenom))
		if tally < 5 {
			require.True(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))
			require.False(t, input.MarketKeeper.IsOracleHalted(input.Ctx))
		}
	}

	require.False(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))
	require.True(t, input.MarketKeeper.IsOracleHalted(input.Ctx))
	require.Equal(t, markettypes.OracleQuorumMissedBlockLimit, input.MarketKeeper.GetOracleMissedBlocks(
		input.Ctx, oracletypes.MetaUSDDenom,
	))

	haltEvents := 0
	for _, event := range input.Ctx.EventManager().Events() {
		if event.Type == markettypes.EventOracleHalt {
			haltEvents++
		}
	}
	require.Equal(t, 1, haltEvents)

	// Further missed periods keep the persisted counter capped and do not emit
	// duplicate transition events.
	input.MarketKeeper.AfterOracleTally(input.Ctx, 10, ustcVotePowers(0, 80, 100))
	require.Equal(t, markettypes.OracleQuorumMissedBlockLimit, input.MarketKeeper.GetOracleMissedBlocks(
		input.Ctx, oracletypes.MetaUSDDenom,
	))
	haltEvents = 0
	for _, event := range input.Ctx.EventManager().Events() {
		if event.Type == markettypes.EventOracleHalt {
			haltEvents++
		}
	}
	require.Equal(t, 1, haltEvents)
}

func TestOracleQuorumExactlyHalfIsHealthyAndResetsCounter(t *testing.T) {
	input := CreateTestInput(t)
	input.MarketKeeper.SetMarketAssets(defaultMarketAssets())
	input.MarketKeeper.SetOracleMissedBlocks(input.Ctx, oracletypes.MetaUSDDenom, 20)

	input.MarketKeeper.AfterOracleTally(input.Ctx, 5, ustcVotePowers(50, 50, 100))

	require.True(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))
	require.False(t, input.MarketKeeper.IsOracleHalted(input.Ctx))
	require.Zero(t, input.MarketKeeper.GetOracleMissedBlocks(input.Ctx, oracletypes.MetaUSDDenom))
	require.Zero(t, input.MarketKeeper.GetOracleMissedBlocks(input.Ctx, core.MicroUSDDenom))
}

func TestOracleQuorumRecoveryRequiresEveryMarketInput(t *testing.T) {
	input := CreateTestInput(t)
	input.MarketKeeper.SetMarketAssets(defaultMarketAssets())
	input.Ctx = input.Ctx.WithEventManager(sdk.NewEventManager())

	input.MarketKeeper.AfterOracleTally(input.Ctx, 25, ustcVotePowers(0, 100, 100))
	require.True(t, input.MarketKeeper.IsOracleHalted(input.Ctx))

	// The original failing denom recovers, but the other required input is now
	// unhealthy. Market must remain disabled until both are healthy together.
	input.MarketKeeper.AfterOracleTally(input.Ctx, 5, ustcVotePowers(100, 49, 100))
	require.True(t, input.MarketKeeper.IsOracleHalted(input.Ctx))
	require.False(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))
	require.Zero(t, input.MarketKeeper.GetOracleMissedBlocks(input.Ctx, oracletypes.MetaUSDDenom))
	require.Equal(t, uint64(5), input.MarketKeeper.GetOracleMissedBlocks(
		input.Ctx, core.MicroUSDDenom,
	))

	input.MarketKeeper.AfterOracleTally(input.Ctx, 5, ustcVotePowers(70, 80, 100))
	require.False(t, input.MarketKeeper.IsOracleHalted(input.Ctx))
	require.True(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))
	require.Empty(t, input.MarketKeeper.GetOracleQuorumStates(input.Ctx))

	recoveryEvents := 0
	for _, event := range input.Ctx.EventManager().Events() {
		if event.Type == markettypes.EventOracleRecovery {
			recoveryEvents++
		}
	}
	require.Equal(t, 1, recoveryEvents)
}

func TestOracleRecoveryDoesNotBypassInitialActivation(t *testing.T) {
	input := CreateTestInput(t)
	input.MarketKeeper.SetMarketAssets(defaultMarketAssets())
	input.MarketKeeper.SetMarketEnabled(input.Ctx, false)
	input.MarketKeeper.SetInitialActivationPending(input.Ctx, true)
	input.MarketKeeper.SetOracleHalted(input.Ctx, true)

	input.MarketKeeper.AfterOracleTally(input.Ctx, 5, ustcVotePowers(100, 100, 100))

	require.False(t, input.MarketKeeper.IsOracleHalted(input.Ctx))
	require.False(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))
	require.True(t, input.MarketKeeper.IsInitialActivationPending(input.Ctx))
}

func TestOracleRecoveryDoesNotOverrideIndependentDisable(t *testing.T) {
	input := CreateTestInput(t)
	input.MarketKeeper.SetMarketAssets(defaultMarketAssets())
	input.MarketKeeper.SetMarketEnabled(input.Ctx, false)
	input.MarketKeeper.SetInitialActivationPending(input.Ctx, false)
	input.MarketKeeper.SetOracleHalted(input.Ctx, true)

	input.MarketKeeper.AfterOracleTally(input.Ctx, 5, ustcVotePowers(100, 100, 100))

	require.False(t, input.MarketKeeper.IsOracleHalted(input.Ctx))
	require.False(t, input.MarketKeeper.IsMarketActivationEnabled(input.Ctx))
	require.False(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))
}

func TestInitialActivationWaitsWhileOracleIsHalted(t *testing.T) {
	input := CreateTestInput(t)
	input.MarketKeeper.SetMarketAssets(defaultMarketAssets())
	input.MarketKeeper.SetMarketEnabled(input.Ctx, false)
	input.MarketKeeper.SetInitialActivationPending(input.Ctx, true)
	input.MarketKeeper.SetOracleHalted(input.Ctx, true)

	liquidity := sdk.NewCoins(
		sdk.NewInt64Coin(core.MicroLunaDenom, 1_000_000),
		sdk.NewInt64Coin(core.MicroUSDDenom, 1_000_000),
	)
	require.NoError(t, FundModuleAccount(input, markettypes.ModuleName, liquidity))

	input.MarketKeeper.activateAfterInitialEpoch(input.Ctx)
	require.False(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))
	require.True(t, input.MarketKeeper.IsInitialActivationPending(input.Ctx))

	input.MarketKeeper.AfterOracleTally(input.Ctx, 5, ustcVotePowers(100, 100, 100))
	input.MarketKeeper.activateAfterInitialEpoch(input.Ctx)
	require.True(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))
	require.False(t, input.MarketKeeper.IsInitialActivationPending(input.Ctx))
}

func TestInitialEpochBoundaryIsRetriedAfterOracleRecovery(t *testing.T) {
	input := CreateTestInput(t)
	input.MarketKeeper.SetMarketAssets(defaultMarketAssets())
	params := input.MarketKeeper.GetParams(input.Ctx)
	params.EpochLengthBlocks = 10
	input.MarketKeeper.SetParams(input.Ctx, params)

	store := input.Ctx.KVStore(input.MarketKeeper.storeKey)
	store.Delete(markettypes.MarketEnabledKey)
	store.Delete(markettypes.InitialActivationPendingKey)
	store.Delete(markettypes.OracleHaltedKey)
	store.Delete(markettypes.EpochLastHeightKey)
	input.Ctx = input.Ctx.WithBlockHeight(100)
	input.MarketKeeper.InitializeMarketForUpgrade(input.Ctx)
	input.MarketKeeper.SetOracleHalted(input.Ctx, true)

	liquidity := sdk.NewCoins(
		sdk.NewInt64Coin(core.MicroLunaDenom, 1_000_000),
		sdk.NewInt64Coin(core.MicroUSDDenom, 1_000_000),
	)
	require.NoError(t, input.BankKeeper.MintCoins(input.Ctx, faucetAccountName, liquidity))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(
		input.Ctx, faucetAccountName, markettypes.AccumulatorModuleName, liquidity,
	))
	setFreshAdaptiveOracle(input)

	input.Ctx = input.Ctx.WithBlockHeight(110)
	input.MarketKeeper.ProcessEpochIfDue(input.Ctx)
	require.Equal(t, int64(100), input.MarketKeeper.GetLastEpochHeight(input.Ctx))
	require.Equal(t, liquidity, input.BankKeeper.GetAllBalances(
		input.Ctx, input.AccountKeeper.GetModuleAddress(markettypes.AccumulatorModuleName),
	))
	require.False(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))

	input.MarketKeeper.AfterOracleTally(input.Ctx, 5, ustcVotePowers(100, 100, 100))
	input.MarketKeeper.ProcessEpochIfDue(input.Ctx)
	require.Equal(t, int64(110), input.MarketKeeper.GetLastEpochHeight(input.Ctx))
	require.True(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))
	require.False(t, input.MarketKeeper.IsInitialActivationPending(input.Ctx))
}

func TestOracleQuorumUsesGenericAssetRegistry(t *testing.T) {
	input := CreateTestInput(t)
	input.MarketKeeper.SetMarketAssets([]MarketAssetConfig{testEUTCAsset()})

	input.MarketKeeper.AfterOracleTally(input.Ctx, 25, []oracletypes.DenomVotePower{
		{Denom: core.MicroUSDDenom, VotePower: 80, TotalPower: 100},
		{Denom: testEUTCOracleDenom, VotePower: 49, TotalPower: 100},
		// A healthy UST observation is irrelevant when USTC is not configured.
		{Denom: oracletypes.MetaUSDDenom, VotePower: 100, TotalPower: 100},
	})

	require.True(t, input.MarketKeeper.IsOracleHalted(input.Ctx))
	require.Equal(t, markettypes.OracleQuorumMissedBlockLimit, input.MarketKeeper.GetOracleMissedBlocks(
		input.Ctx, testEUTCOracleDenom,
	))
	require.Zero(t, input.MarketKeeper.GetOracleMissedBlocks(input.Ctx, oracletypes.MetaUSDDenom))
}

func TestMissingOracleObservationCountsAsNoQuorum(t *testing.T) {
	input := CreateTestInput(t)
	input.MarketKeeper.SetMarketAssets(defaultMarketAssets())

	input.MarketKeeper.AfterOracleTally(input.Ctx, 25, []oracletypes.DenomVotePower{
		{Denom: core.MicroUSDDenom, VotePower: 100, TotalPower: 100},
	})

	require.True(t, input.MarketKeeper.IsOracleHalted(input.Ctx))
	require.Equal(t, markettypes.OracleQuorumMissedBlockLimit, input.MarketKeeper.GetOracleMissedBlocks(
		input.Ctx, oracletypes.MetaUSDDenom,
	))
}
