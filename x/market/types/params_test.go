package types

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func TestParamsEqual(t *testing.T) {
	p1 := DefaultParams()
	err := p1.Validate()
	require.NoError(t, err)

	// invalid base pool
	p1.BasePool = sdkmath.LegacyNewDec(-1)
	err = p1.Validate()
	require.Error(t, err)

	// invalid pool recovery period
	p3 := DefaultParams()
	p3.PoolRecoveryPeriod = 0
	err = p3.Validate()
	require.Error(t, err)

	// invalid min spread
	p4 := DefaultParams()
	p4.MinStabilitySpread = sdkmath.LegacyNewDecWithPrec(-1, 2)
	err = p4.Validate()
	require.Error(t, err)

	p5 := DefaultParams()
	require.NotNil(t, p5.ParamSetPairs())
	require.NotNil(t, p5.String())

	// The direct cap may be tightened but never raised above the proposal's
	// absolute 10% daily ceiling.
	p6 := DefaultParams()
	p6.DailyCapFactor = sdkmath.LegacyMustNewDecFromStr("0.11")
	require.Error(t, p6.Validate())
	p6.DailyCapFactor = sdkmath.LegacyMustNewDecFromStr("0.07")
	require.NoError(t, p6.Validate())
}
