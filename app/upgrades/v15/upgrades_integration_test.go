package v15_test

import (
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	apptesting "github.com/classic-terra/core/v4/app/testing"
	v15 "github.com/classic-terra/core/v4/app/upgrades/v15"
	core "github.com/classic-terra/core/v4/types"
	markettypes "github.com/classic-terra/core/v4/x/market/types"
	oracletypes "github.com/classic-terra/core/v4/x/oracle/types"
	treasurytypes "github.com/classic-terra/core/v4/x/treasury/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/stretchr/testify/require"
)

func TestUpgradeInitializesPreMM2StateAndIsIdempotent(t *testing.T) {
	terraApp := apptesting.SetupApp(t, "mm2-v15-upgrade-test")
	ctx := terraApp.NewUncachedContext(false, tmproto.Header{
		Height: 100,
		Time:   time.Now().UTC(),
	})

	legacyMarketParams := markettypes.DefaultParams()
	legacyMarketParams.BasePool = sdkmath.LegacyNewDec(987_654_321)
	legacyMarketParams.PoolRecoveryPeriod = 91_234
	legacyMarketParams.MinStabilitySpread = sdkmath.LegacyOneDec()
	terraApp.MarketKeeper.SetParams(ctx, legacyMarketParams)
	terraApp.TreasuryKeeper.SetParams(ctx, treasurytypes.DefaultParams())
	terraApp.OracleKeeper.SetParams(ctx, oracletypes.DefaultParams())
	legacyGovParams := govv1.DefaultParams()
	require.NoError(t, terraApp.GovKeeper.Params.Set(ctx, legacyGovParams))

	marketSubspace := terraApp.GetSubspace(markettypes.ModuleName)
	for _, key := range [][]byte{
		markettypes.KeyEpochLengthBlocks,
		markettypes.KeySwapFeeBurnRate,
		markettypes.KeySwapFeeCommunityRate,
		markettypes.KeyMaxOracleAgeSeconds,
		markettypes.KeyTWAPLookbackWindow,
		markettypes.KeyMaxTWAPDeviation,
		markettypes.KeyDailyCapFactor,
	} {
		deleteSubspaceParam(ctx, terraApp.GetKey(paramstypes.StoreKey), markettypes.ModuleName, key)
		require.False(t, marketSubspace.Has(ctx, key))
	}

	deleteSubspaceParam(ctx, terraApp.GetKey(paramstypes.StoreKey), treasurytypes.ModuleName, treasurytypes.KeyTaxRedirectRate)
	require.False(t, terraApp.GetSubspace(treasurytypes.ModuleName).Has(ctx, treasurytypes.KeyTaxRedirectRate))

	marketStore := ctx.KVStore(terraApp.GetKey(markettypes.StoreKey))
	marketStore.Delete(markettypes.MarketEnabledKey)
	marketStore.Delete(markettypes.InitialActivationPendingKey)
	marketStore.Delete(markettypes.OracleHaltedKey)
	marketStore.Delete(markettypes.EpochLastHeightKey)

	oracleParams := terraApp.OracleKeeper.GetParams(ctx)
	oracleParams.Whitelist = removeOracleDenom(oracleParams.Whitelist, oracletypes.MetaUSDDenom)
	terraApp.OracleKeeper.SetParams(ctx, oracleParams)

	manager := module.NewManager()
	configurator := module.NewConfigurator(
		terraApp.AppCodec(),
		terraApp.MsgServiceRouter(),
		terraApp.GRPCQueryRouter(),
	)
	handler := v15.CreateV15UpgradeHandler(manager, configurator, nil, terraApp.AppKeepers)
	versionMap, err := handler(
		sdk.WrapSDKContext(ctx),
		upgradetypes.Plan{Name: v15.UpgradeName, Height: ctx.BlockHeight()},
		module.VersionMap{},
	)
	require.NoError(t, err)
	require.Empty(t, versionMap)

	migrated := terraApp.MarketKeeper.GetParams(ctx)
	require.Equal(t, legacyMarketParams.BasePool, migrated.BasePool)
	require.Equal(t, legacyMarketParams.PoolRecoveryPeriod, migrated.PoolRecoveryPeriod)
	require.Equal(t, sdkmath.LegacyMustNewDecFromStr("0.0035"), migrated.MinStabilitySpread)
	require.Equal(t, markettypes.DefaultEpochLengthBlocks, migrated.EpochLengthBlocks)
	require.Equal(t, sdkmath.LegacyMustNewDecFromStr("0.5"), migrated.SwapFeeBurnRate)
	require.True(t, migrated.SwapFeeCommunityRate.IsZero())
	require.Equal(t, markettypes.DefaultMaxOracleAgeSeconds, migrated.MaxOracleAgeSeconds)
	require.Equal(t, markettypes.DefaultTWAPLookbackWindow, migrated.TwapLookbackWindow)
	require.Equal(t, markettypes.DefaultMaxTWAPDeviation, migrated.MaxTwapDeviation)
	require.Equal(t, markettypes.DefaultDailyCapFactor, migrated.DailyCapFactor)
	require.Equal(t, treasurytypes.DefaultTaxRedirectRate, terraApp.TreasuryKeeper.GetTaxRedirectRate(ctx))
	migratedGovParams, err := terraApp.GovKeeper.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, core.MicroLunaDenom, migratedGovParams.MinDeposit[0].Denom)
	require.Equal(t, core.MicroLunaDenom, migratedGovParams.ExpeditedMinDeposit[0].Denom)
	require.Equal(t, legacyGovParams.ExpeditedMinDeposit[0].Amount, migratedGovParams.ExpeditedMinDeposit[0].Amount)
	require.Equal(t, govv1.DefaultExpeditedThreshold.String(), migratedGovParams.ExpeditedThreshold)

	require.False(t, terraApp.MarketKeeper.IsMarketEnabled(ctx))
	require.True(t, terraApp.MarketKeeper.IsInitialActivationPending(ctx))
	require.False(t, terraApp.MarketKeeper.IsOracleHalted(ctx))
	require.True(t, marketStore.Has(markettypes.OracleHaltedKey))
	require.Equal(t, int64(100), terraApp.MarketKeeper.GetLastEpochHeight(ctx))
	require.True(t, hasOracleDenom(terraApp.OracleKeeper.GetParams(ctx).Whitelist, oracletypes.MetaUSDDenom))

	retryCtx := ctx.WithBlockHeight(105)
	_, err = handler(
		sdk.WrapSDKContext(retryCtx),
		upgradetypes.Plan{Name: v15.UpgradeName, Height: retryCtx.BlockHeight()},
		module.VersionMap{},
	)
	require.NoError(t, err)
	require.False(t, terraApp.MarketKeeper.IsMarketEnabled(retryCtx))
	require.True(t, terraApp.MarketKeeper.IsInitialActivationPending(retryCtx))
	require.Equal(t, int64(100), terraApp.MarketKeeper.GetLastEpochHeight(retryCtx))
}

func deleteSubspaceParam(ctx sdk.Context, paramsStoreKey storetypes.StoreKey, moduleName string, key []byte) {
	storeKey := append([]byte(moduleName+"/"), key...)
	ctx.KVStore(paramsStoreKey).Delete(storeKey)
}

func removeOracleDenom(whitelist oracletypes.DenomList, denom string) oracletypes.DenomList {
	result := make(oracletypes.DenomList, 0, len(whitelist))
	for _, item := range whitelist {
		if item.Name != denom {
			result = append(result, item)
		}
	}
	return result
}

func hasOracleDenom(whitelist oracletypes.DenomList, denom string) bool {
	for _, item := range whitelist {
		if item.Name == denom {
			return true
		}
	}
	return false
}
