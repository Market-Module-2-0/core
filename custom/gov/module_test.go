package gov

import (
	"testing"

	core "github.com/classic-terra/core/v4/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/stretchr/testify/require"
)

func TestDefaultGenesisUsesULunaForEveryProposalPath(t *testing.T) {
	cdc := codec.NewProtoCodec(types.NewInterfaceRegistry())
	genesis := govv1.GenesisState{}
	cdc.MustUnmarshalJSON(AppModuleBasic{}.DefaultGenesis(cdc), &genesis)

	require.Len(t, genesis.Params.MinDeposit, 1)
	require.Equal(t, core.MicroLunaDenom, genesis.Params.MinDeposit[0].Denom)
	require.Len(t, genesis.Params.ExpeditedMinDeposit, 1)
	require.Equal(t, core.MicroLunaDenom, genesis.Params.ExpeditedMinDeposit[0].Denom)
	require.NoError(t, genesis.Params.ValidateBasic())
}
