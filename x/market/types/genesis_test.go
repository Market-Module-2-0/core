package types

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func TestGenesisValidation(t *testing.T) {
	genState := DefaultGenesisState()
	require.NoError(t, ValidateGenesis(genState))
	require.True(t, genState.MarketEnabled)
	require.False(t, genState.InitialActivationPending)
	require.False(t, genState.OracleHalted)
	require.Empty(t, genState.OracleQuorumStates)

	genState.Params.BasePool = sdkmath.LegacyNewDec(-1)
	require.Error(t, ValidateGenesis(genState))

	genState = DefaultGenesisState()
	genState.Params.PoolRecoveryPeriod = 0
	require.Error(t, ValidateGenesis(genState))

	genState = DefaultGenesisState()
	genState.Params.MinStabilitySpread = sdkmath.LegacyNewDec(-1)
	require.Error(t, ValidateGenesis(genState))

	genState = DefaultGenesisState()
	genState.InitialActivationPending = true
	require.Error(t, ValidateGenesis(genState))

	genState = DefaultGenesisState()
	genState.LastEpochHeight = -1
	require.Error(t, ValidateGenesis(genState))

	genState = DefaultGenesisState()
	genState.OracleHalted = true
	require.NoError(t, ValidateGenesis(genState))

	genState = DefaultGenesisState()
	genState.MarketEnabled = false
	genState.OracleQuorumStates = []OracleQuorumState{{
		OracleDenom:             "uusd",
		ConsecutiveMissedBlocks: 25,
	}}
	require.NoError(t, ValidateGenesis(genState))

	genState.OracleQuorumStates = append(genState.OracleQuorumStates, OracleQuorumState{
		OracleDenom:             "uusd",
		ConsecutiveMissedBlocks: 5,
	})
	require.Error(t, ValidateGenesis(genState))
}
