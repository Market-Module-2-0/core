package v15

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	core "github.com/classic-terra/core/v4/types"
	markettypes "github.com/classic-terra/core/v4/x/market/types"
	treasurytypes "github.com/classic-terra/core/v4/x/treasury/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/stretchr/testify/require"
)

func TestMigrateMarketParams(t *testing.T) {
	current := markettypes.DefaultParams()
	current.BasePool = sdkmath.LegacyNewDec(987_654_321)
	current.PoolRecoveryPeriod = 91_234
	current.MinStabilitySpread = sdkmath.LegacyOneDec()
	current.SwapFeeBurnRate = sdkmath.LegacyZeroDec()

	migrated := migrateMarketParams(current)
	require.Equal(t, current.BasePool, migrated.BasePool)
	require.Equal(t, current.PoolRecoveryPeriod, migrated.PoolRecoveryPeriod)
	require.Equal(t, sdkmath.LegacyMustNewDecFromStr("0.0035"), migrated.MinStabilitySpread)
	require.Equal(t, uint64(30*core.BlocksPerDay), migrated.EpochLengthBlocks)
	require.Equal(t, sdkmath.LegacyMustNewDecFromStr("0.5"), migrated.SwapFeeBurnRate)
	require.True(t, migrated.SwapFeeCommunityRate.IsZero())
	require.Equal(t, uint64(75), migrated.MaxOracleAgeSeconds)
	require.Equal(t, uint64(45), migrated.TwapLookbackWindow)
	require.Equal(t, sdkmath.LegacyMustNewDecFromStr("0.1"), migrated.MaxTwapDeviation)
	require.Equal(t, sdkmath.LegacyMustNewDecFromStr("0.1"), migrated.DailyCapFactor)
	require.Equal(t, migrated, migrateMarketParams(migrated))
}

func TestMigrateMarketParamsFromMissingState(t *testing.T) {
	migrated := migrateMarketParams(markettypes.Params{})
	require.False(t, migrated.BasePool.IsNil())
	require.NotZero(t, migrated.PoolRecoveryPeriod)
	require.NoError(t, migrated.Validate())
}

func TestMigrateTreasuryParams(t *testing.T) {
	current := treasurytypes.DefaultParams()
	current.OracleSplit = sdkmath.LegacyMustNewDecFromStr("0.42")
	current.TaxRedirectRate = sdkmath.LegacyZeroDec()

	migrated := migrateTreasuryParams(current)
	require.Equal(t, current.OracleSplit, migrated.OracleSplit)
	require.Equal(t, sdkmath.LegacyMustNewDecFromStr("0.6"), migrated.TaxRedirectRate)
	require.Equal(t, migrated, migrateTreasuryParams(migrated))
}

func TestMigrateGovernanceParams(t *testing.T) {
	current := govv1.DefaultParams()
	regularAmount := current.MinDeposit[0].Amount
	expeditedAmount := current.ExpeditedMinDeposit[0].Amount

	migrated := migrateGovernanceParams(current)
	require.Equal(t, sdk.NewCoins(sdk.NewCoin(core.MicroLunaDenom, regularAmount)), sdk.Coins(migrated.MinDeposit))
	require.Equal(t, sdk.NewCoins(sdk.NewCoin(core.MicroLunaDenom, expeditedAmount)), sdk.Coins(migrated.ExpeditedMinDeposit))
	require.Equal(t, govv1.DefaultExpeditedThreshold.String(), migrated.ExpeditedThreshold)
	require.NoError(t, migrated.ValidateBasic())
	require.Equal(t, migrated, migrateGovernanceParams(migrated))
}
