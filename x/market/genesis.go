package market

import (
	"fmt"

	"github.com/classic-terra/core/v4/x/market/keeper"
	"github.com/classic-terra/core/v4/x/market/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initialize default parameters
// and the keeper's address to pubkey map
func InitGenesis(ctx sdk.Context, keeper keeper.Keeper, data *types.GenesisState) {
	keeper.SetParams(ctx, data.Params)
	keeper.SetTerraPoolDelta(ctx, data.TerraPoolDelta)
	keeper.SetMarketEnabled(ctx, data.MarketEnabled)
	keeper.SetInitialActivationPending(ctx, data.InitialActivationPending)
	keeper.SetLastEpochHeight(ctx, data.LastEpochHeight)
	keeper.SetOracleHalted(ctx, data.OracleHalted)
	keeper.ClearOracleQuorumStates(ctx)
	for _, state := range data.OracleQuorumStates {
		keeper.SetOracleMissedBlocks(ctx, state.OracleDenom, state.ConsecutiveMissedBlocks)
	}

	// check if the module account exists
	moduleAcc := keeper.GetMarketAccount(ctx)
	if moduleAcc == nil {
		panic(fmt.Sprintf("%s module account has not been set", types.ModuleName))
	}

	accumulatorAcc := keeper.EnsureMarketAccumulatorAccount(ctx)
	if accumulatorAcc == nil {
		panic(fmt.Sprintf("%s module account has not been set", types.AccumulatorModuleName))
	}
}

// ExportGenesis writes the current store values
// to a genesis file, which can be imported again
// with InitGenesis
func ExportGenesis(ctx sdk.Context, keeper keeper.Keeper) (data *types.GenesisState) {
	params := keeper.GetParams(ctx)
	terraPoolDelta := keeper.GetTerraPoolDelta(ctx)
	marketEnabled := keeper.IsMarketActivationEnabled(ctx)
	initialActivationPending := keeper.IsInitialActivationPending(ctx)
	lastEpochHeight := keeper.GetLastEpochHeight(ctx)
	oracleHalted := keeper.IsOracleHalted(ctx)
	oracleQuorumStates := keeper.GetOracleQuorumStates(ctx)

	data = types.NewGenesisState(terraPoolDelta, params)
	data.MarketEnabled = marketEnabled
	data.InitialActivationPending = initialActivationPending
	data.LastEpochHeight = lastEpochHeight
	data.OracleHalted = oracleHalted
	data.OracleQuorumStates = oracleQuorumStates
	return data
}
