package types

import (
	"encoding/json"
	"fmt"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
)

// NewGenesisState creates a new GenesisState object
func NewGenesisState(terraPoolDelta math.LegacyDec, params Params) *GenesisState {
	return &GenesisState{
		TerraPoolDelta:           terraPoolDelta,
		Params:                   params,
		MarketEnabled:            true,
		InitialActivationPending: false,
		LastEpochHeight:          0,
		OracleHalted:             false,
		OracleQuorumStates:       nil,
	}
}

// DefaultGenesisState returns raw genesis raw message for testing
func DefaultGenesisState() *GenesisState {
	return NewGenesisState(math.LegacyZeroDec(), DefaultParams())
}

// ValidateGenesis validates the provided market genesis state
func ValidateGenesis(data *GenesisState) error {
	if data.MarketEnabled && data.InitialActivationPending {
		return fmt.Errorf("market cannot be enabled while initial activation is pending")
	}
	if data.LastEpochHeight < 0 {
		return fmt.Errorf("last epoch height cannot be negative: %d", data.LastEpochHeight)
	}
	seenOracleDenoms := make(map[string]struct{}, len(data.OracleQuorumStates))
	for _, state := range data.OracleQuorumStates {
		if state.OracleDenom == "" {
			return fmt.Errorf("oracle quorum denom cannot be empty")
		}
		if state.ConsecutiveMissedBlocks == 0 || state.ConsecutiveMissedBlocks > OracleQuorumMissedBlockLimit {
			return fmt.Errorf(
				"invalid consecutive missed blocks for %s: %d",
				state.OracleDenom,
				state.ConsecutiveMissedBlocks,
			)
		}
		if _, exists := seenOracleDenoms[state.OracleDenom]; exists {
			return fmt.Errorf("duplicate oracle quorum denom: %s", state.OracleDenom)
		}
		seenOracleDenoms[state.OracleDenom] = struct{}{}
	}
	return data.Params.Validate()
}

// GetGenesisStateFromAppState returns x/market GenesisState given raw application
// genesis state.
func GetGenesisStateFromAppState(cdc codec.JSONCodec, appState map[string]json.RawMessage) *GenesisState {
	var genesisState GenesisState

	if appState[ModuleName] != nil {
		cdc.MustUnmarshalJSON(appState[ModuleName], &genesisState)
	}

	return &genesisState
}
