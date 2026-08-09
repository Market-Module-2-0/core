package market

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	core "github.com/classic-terra/core/v4/types"
	"github.com/classic-terra/core/v4/x/market/keeper"
	"github.com/classic-terra/core/v4/x/market/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"
)

func TestExportInitGenesis(t *testing.T) {
	input := keeper.CreateTestInput(t)
	params := input.MarketKeeper.GetParams(input.Ctx)
	params.MinStabilitySpread = sdkmath.LegacyOneDec()
	input.MarketKeeper.SetParams(input.Ctx, params)
	input.MarketKeeper.SetTerraPoolDelta(input.Ctx, sdkmath.LegacyNewDec(1123))
	input.MarketKeeper.SetMarketEnabled(input.Ctx, false)
	input.MarketKeeper.SetInitialActivationPending(input.Ctx, true)
	input.MarketKeeper.SetLastEpochHeight(input.Ctx, 42)
	input.MarketKeeper.SetOracleHalted(input.Ctx, true)
	input.MarketKeeper.SetOracleMissedBlocks(input.Ctx, core.MicroUSDDenom, 25)
	input.MarketKeeper.SetOracleMissedBlocks(input.Ctx, "UST", 10)
	genesis := ExportGenesis(input.Ctx, input.MarketKeeper)

	newInput := keeper.CreateTestInput(t)
	InitGenesis(newInput.Ctx, newInput.MarketKeeper, genesis)
	newGenesis := ExportGenesis(newInput.Ctx, newInput.MarketKeeper)

	require.Equal(t, genesis, newGenesis)
	require.Equal(t, sdkmath.LegacyOneDec(), newInput.MarketKeeper.MinStabilitySpread(newInput.Ctx))
}

func TestExportPreservesBaseActivationDuringOracleHalt(t *testing.T) {
	input := keeper.CreateTestInput(t)
	input.MarketKeeper.SetMarketEnabled(input.Ctx, true)
	input.MarketKeeper.SetOracleHalted(input.Ctx, true)
	require.True(t, input.MarketKeeper.IsMarketActivationEnabled(input.Ctx))
	require.False(t, input.MarketKeeper.IsMarketEnabled(input.Ctx))

	genesis := ExportGenesis(input.Ctx, input.MarketKeeper)
	require.True(t, genesis.MarketEnabled)
	require.True(t, genesis.OracleHalted)

	newInput := keeper.CreateTestInput(t)
	InitGenesis(newInput.Ctx, newInput.MarketKeeper, genesis)
	require.True(t, newInput.MarketKeeper.IsMarketActivationEnabled(newInput.Ctx))
	require.False(t, newInput.MarketKeeper.IsMarketEnabled(newInput.Ctx))
}

func TestInitGenesisCreatesMarketAccumulatorAccount(t *testing.T) {
	input := keeper.CreateTestInput(t)
	accumulatorAddr := input.AccountKeeper.GetModuleAddress(types.AccumulatorModuleName)
	input.AccountKeeper.RemoveAccount(input.Ctx, input.AccountKeeper.GetAccount(input.Ctx, accumulatorAddr))
	require.Nil(t, input.AccountKeeper.GetAccount(input.Ctx, accumulatorAddr))

	genesis := ExportGenesis(input.Ctx, input.MarketKeeper)
	InitGenesis(input.Ctx, input.MarketKeeper, genesis)

	account := input.AccountKeeper.GetAccount(input.Ctx, accumulatorAddr)
	moduleAccount, ok := account.(authtypes.ModuleAccountI)
	require.True(t, ok)
	require.Equal(t, types.AccumulatorModuleName, moduleAccount.GetName())
	require.Empty(t, moduleAccount.GetPermissions())
}

func TestInitGenesisConvertsAccumulatorBaseAccount(t *testing.T) {
	input := keeper.CreateTestInput(t)
	accumulatorAddr := input.AccountKeeper.GetModuleAddress(types.AccumulatorModuleName)
	input.AccountKeeper.RemoveAccount(input.Ctx, input.AccountKeeper.GetAccount(input.Ctx, accumulatorAddr))

	const accountNumber uint64 = 77
	const sequence uint64 = 3
	baseAccount := authtypes.NewBaseAccount(accumulatorAddr, nil, accountNumber, sequence)
	input.AccountKeeper.SetAccount(input.Ctx, baseAccount)
	accumulatorBalance := sdk.NewCoins(sdk.NewCoin(core.MicroUSDDenom, sdkmath.NewInt(12_345)))
	require.NoError(t, keeper.FundAccount(input, accumulatorAddr, accumulatorBalance))

	genesis := ExportGenesis(input.Ctx, input.MarketKeeper)
	InitGenesis(input.Ctx, input.MarketKeeper, genesis)
	InitGenesis(input.Ctx, input.MarketKeeper, genesis)

	account := input.AccountKeeper.GetAccount(input.Ctx, accumulatorAddr)
	moduleAccount, ok := account.(authtypes.ModuleAccountI)
	require.True(t, ok)
	require.Equal(t, types.AccumulatorModuleName, moduleAccount.GetName())
	require.Equal(t, accountNumber, moduleAccount.GetAccountNumber())
	require.Equal(t, sequence, moduleAccount.GetSequence())
	require.Equal(t, accumulatorBalance, input.BankKeeper.GetAllBalances(input.Ctx, accumulatorAddr))
}
