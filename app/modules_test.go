package app

import (
	"testing"

	sdklog "cosmossdk.io/log"
	core "github.com/classic-terra/core/v4/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestModuleOrdersOnlyReferenceRegisteredModules(t *testing.T) {
	sdk.GetConfig().SetBech32PrefixForAccount(core.Bech32PrefixAccAddr, core.Bech32PrefixAccPub)
	sdk.GetConfig().SetBech32PrefixForValidator(core.Bech32PrefixValAddr, core.Bech32PrefixValPub)
	sdk.GetConfig().SetBech32PrefixForConsensusNode(core.Bech32PrefixConsAddr, core.Bech32PrefixConsPub)

	encodingConfig := MakeEncodingConfig()
	app := NewTerraApp(
		sdklog.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		map[int64]bool{},
		t.TempDir(),
		encodingConfig,
		sims.EmptyAppOptions{},
		nil,
		baseapp.SetChainID("module-order-test"),
	)

	orders := map[string][]string{
		"begin blockers": orderBeginBlockers(),
		"end blockers":   orderEndBlockers(),
		"init genesis":   orderInitGenesis(),
		"export genesis": app.mm.OrderExportGenesis,
	}
	for orderName, order := range orders {
		t.Run(orderName, func(t *testing.T) {
			for _, moduleName := range order {
				require.Containsf(t, app.mm.Modules, moduleName,
					"%s references an unregistered module", orderName)
			}
		})
	}
}
