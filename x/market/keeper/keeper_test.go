package keeper

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	core "github.com/classic-terra/core/v4/types"
	"github.com/stretchr/testify/require"
)

func TestTerraPoolDeltaUpdate(t *testing.T) {
	input := CreateTestInput(t)

	terraPoolDelta := input.MarketKeeper.GetTerraPoolDelta(input.Ctx)
	require.Equal(t, sdkmath.LegacyZeroDec(), terraPoolDelta)

	diff := sdkmath.LegacyNewDec(10)
	input.MarketKeeper.SetTerraPoolDelta(input.Ctx, diff)

	terraPoolDelta = input.MarketKeeper.GetTerraPoolDelta(input.Ctx)
	require.Equal(t, diff, terraPoolDelta)
}

// TestReplenishPools tests that
// each pools move towards base pool
func TestReplenishPools(t *testing.T) {
	input := CreateTestInput(t)
	input.OracleKeeper.SetLunaExchangeRate(input.Ctx, core.MicroSDRDenom, sdkmath.LegacyOneDec())

	basePool := input.MarketKeeper.BasePool(input.Ctx)
	terraPoolDelta := input.MarketKeeper.GetTerraPoolDelta(input.Ctx)
	require.True(t, terraPoolDelta.IsZero())

	// Positive delta
	diff := basePool.QuoInt64((int64)(core.BlocksPerDay))
	input.MarketKeeper.SetTerraPoolDelta(input.Ctx, diff)

	input.MarketKeeper.ReplenishPools(input.Ctx)

	terraPoolDelta = input.MarketKeeper.GetTerraPoolDelta(input.Ctx)
	replenishAmt := diff.QuoInt64((int64)(input.MarketKeeper.PoolRecoveryPeriod(input.Ctx)))
	expectedDelta := diff.Sub(replenishAmt)
	require.Equal(t, expectedDelta, terraPoolDelta)

	// Negative delta
	diff = diff.Neg()
	input.MarketKeeper.SetTerraPoolDelta(input.Ctx, diff)

	input.MarketKeeper.ReplenishPools(input.Ctx)

	terraPoolDelta = input.MarketKeeper.GetTerraPoolDelta(input.Ctx)
	replenishAmt = diff.QuoInt64((int64)(input.MarketKeeper.PoolRecoveryPeriod(input.Ctx)))
	expectedDelta = diff.Sub(replenishAmt)
	require.Equal(t, expectedDelta, terraPoolDelta)
}

// TestSetAllowedSwapDenomsPropagatesToCopies verifies that updating the allowed swap
// denoms mutates the shared underlying map, so a Keeper copied by value (as happens
// when it is handed to the msg server / module wiring) observes the change too. With
// the previous map-replacement implementation this would silently no-op on the copy.
func TestSetAllowedSwapDenomsPropagatesToCopies(t *testing.T) {
	input := CreateTestInput(t)

	k := input.MarketKeeper
	// Copy by value, mirroring how the keeper is captured by the msg server/module.
	kCopy := k

	k.SetAllowedSwapDenoms([]string{core.MicroSDRDenom})

	require.True(t, kCopy.isAllowedSwapDenom(core.MicroSDRDenom),
		"copy should observe the updated allowed set via the shared map")
	require.False(t, kCopy.isAllowedSwapDenom(core.MicroUSDDenom),
		"cleared denom should no longer be allowed for the copy either")
}
