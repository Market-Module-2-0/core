package keeper_test

import (
	"testing"
	"time"

	apphelpers "github.com/classic-terra/core/v4/app/testing"
	taxtypes "github.com/classic-terra/core/v4/x/tax/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"
)

// TestIsReverseCharge_MissingContextValue verifies that IsReverseCharge does not
// panic when the reverse-charge context key was never set (e.g. a gov- or
// module-initiated message that bypasses the ante handler and the wasm handler).
// A missing value must be treated as "not reverse charge".
func TestIsReverseCharge_MissingContextValue(t *testing.T) {
	chainID := "tax-reverse-charge-missing-ctx"
	app := apphelpers.SetupApp(t, chainID)
	ctx := app.NewUncachedContext(false, tmproto.Header{Height: 1, ChainID: chainID, Time: time.Now().UTC()})

	// No ContextKeyTaxReverseCharge is set on ctx.
	require.NotPanics(t, func() {
		require.False(t, app.TaxKeeper.IsReverseCharge(ctx, false))
	})
}

// TestIsReverseCharge_ExplicitValues verifies the flag is honored when present.
func TestIsReverseCharge_ExplicitValues(t *testing.T) {
	chainID := "tax-reverse-charge-explicit"
	app := apphelpers.SetupApp(t, chainID)
	ctx := app.NewUncachedContext(false, tmproto.Header{Height: 1, ChainID: chainID, Time: time.Now().UTC()})

	trueCtx := ctx.WithValue(taxtypes.ContextKeyTaxReverseCharge, true)
	require.True(t, app.TaxKeeper.IsReverseCharge(trueCtx, false))

	falseCtx := ctx.WithValue(taxtypes.ContextKeyTaxReverseCharge, false)
	require.False(t, app.TaxKeeper.IsReverseCharge(falseCtx, false))
}
