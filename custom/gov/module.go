package gov

import (
	"encoding/json"

	customtypes "github.com/classic-terra/core/v4/custom/gov/types"
	core "github.com/classic-terra/core/v4/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

var _ module.AppModuleBasic = AppModuleBasic{}

// AppModuleBasic defines the basic application module used by the gov module.
type AppModuleBasic struct {
	gov.AppModuleBasic
}

// NewAppModuleBasic creates a new AppModuleBasic object
func NewAppModuleBasic(proposalHandlers []govclient.ProposalHandler) AppModuleBasic {
	return AppModuleBasic{gov.NewAppModuleBasic(proposalHandlers)}
}

// RegisterLegacyAminoCodec registers the gov module's types for the given codec.
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	customtypes.RegisterLegacyAminoCodec(cdc)
	v1.RegisterLegacyAminoCodec(cdc)
}

// DefaultGenesis returns default genesis state as raw bytes for the gov
// module.
func (am AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	// Customize both governance deposit paths for Terra Classic. Leaving the
	// expedited path on the SDK's generic "stake" denom would make accelerated
	// proposals impossible to fund with the chain's native token.
	defaultGenesisState := v1.DefaultGenesisState()
	defaultGenesisState.Params.MinDeposit[0].Denom = core.MicroLunaDenom
	defaultGenesisState.Params.ExpeditedMinDeposit[0].Denom = core.MicroLunaDenom

	return cdc.MustMarshalJSON(defaultGenesisState)
}
