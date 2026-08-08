package keeper

import (
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	core "github.com/classic-terra/core/v4/types"
	markettypes "github.com/classic-terra/core/v4/x/market/types"
	oracletypes "github.com/classic-terra/core/v4/x/oracle/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

const testEUTCOracleDenom = "EUT"

func testEUTCAsset() MarketAssetConfig {
	return MarketAssetConfig{
		BankDenom:   core.MicroEURDenom,
		OracleDenom: testEUTCOracleDenom,
		PriceSource: MarketAssetPriceUSD,
	}
}

func TestDefaultMarketAssetsEnableOnlyUSTC(t *testing.T) {
	input := CreateTestInput(t)
	input.MarketKeeper.SetMarketAssets(defaultMarketAssets())

	assets := input.MarketKeeper.MarketAssets()
	require.Len(t, assets, 1)
	require.Equal(t, core.MicroUSDDenom, assets[0].BankDenom)
	require.Equal(t, oracletypes.MetaUSDDenom, assets[0].OracleDenom)
	require.Equal(t, MarketAssetPriceUSD, assets[0].PriceSource)

	_, enabled := input.MarketKeeper.marketAssetForPair(core.MicroLunaDenom, core.MicroEURDenom)
	require.False(t, enabled, "the generic code must not activate EUTC by default")
}

func TestGenericUSDMarketAssetPriceResolution(t *testing.T) {
	input := CreateTestInput(t)
	input.MarketKeeper.SetMarketAssets([]MarketAssetConfig{testEUTCAsset()})

	// One LUNC is worth $0.0001 and one test EUTC is worth $0.02, therefore
	// one LUNC is worth 0.005 EUTC.
	input.OracleKeeper.SetLunaExchangeRate(
		input.Ctx, core.MicroUSDDenom, sdkmath.LegacyMustNewDecFromStr("0.0001"),
	)
	input.OracleKeeper.SetLunaExchangeRate(
		input.Ctx, testEUTCOracleDenom, sdkmath.LegacyMustNewDecFromStr("0.02"),
	)

	luncToEUTC, err := input.MarketKeeper.ComputeInternalSwap(
		input.Ctx,
		sdk.NewDecCoinFromCoin(sdk.NewInt64Coin(core.MicroLunaDenom, core.MicroUnit)),
		core.MicroEURDenom,
	)
	require.NoError(t, err)
	require.Equal(t, sdkmath.LegacyNewDec(5_000), luncToEUTC.Amount)

	eutcToLUNC, err := input.MarketKeeper.ComputeInternalSwap(
		input.Ctx,
		sdk.NewDecCoinFromCoin(sdk.NewInt64Coin(core.MicroEURDenom, core.MicroUnit)),
		core.MicroLunaDenom,
	)
	require.NoError(t, err)
	require.Equal(t, sdkmath.LegacyNewDec(200_000_000), eutcToLUNC.Amount)

	_, enabled := input.MarketKeeper.marketAssetForPair(core.MicroLunaDenom, core.MicroEURDenom)
	require.True(t, enabled)
	_, enabled = input.MarketKeeper.marketAssetForPair(core.MicroUSDDenom, core.MicroEURDenom)
	require.False(t, enabled, "stable-to-stable swaps remain disabled")
}

func TestAfterOracleTallyTracksConfiguredAssetInputs(t *testing.T) {
	input := CreateTestInput(t)
	input.MarketKeeper.SetMarketAssets([]MarketAssetConfig{testEUTCAsset()})
	input.Ctx = input.Ctx.WithBlockHeight(50).WithBlockTime(time.Unix(2_000_000, 0))

	prices := map[string]sdkmath.LegacyDec{
		core.MicroUSDDenom:  sdkmath.LegacyMustNewDecFromStr("0.0001"),
		core.MicroSDRDenom:  sdkmath.LegacyMustNewDecFromStr("0.00008"),
		testEUTCOracleDenom: sdkmath.LegacyMustNewDecFromStr("0.02"),
	}
	for denom, price := range prices {
		input.OracleKeeper.SetLunaExchangeRate(input.Ctx, denom, price)
	}

	input.MarketKeeper.AfterOracleTally(input.Ctx, 0, nil)
	for denom, price := range prices {
		snapshots := input.MarketKeeper.GetTWAPPrices(input.Ctx, denom)
		require.Len(t, snapshots, 1, "expected a TWAP snapshot for %s", denom)
		require.Equal(t, price, snapshots[0].Price)
	}
	require.Empty(t, input.MarketKeeper.GetTWAPPrices(input.Ctx, oracletypes.MetaUSDDenom))
}

func TestGenericUSDMarketAssetUsesCompleteSwapPath(t *testing.T) {
	input := CreateTestInput(t)
	input.MarketKeeper.SetMarketAssets([]MarketAssetConfig{testEUTCAsset()})
	input.OracleKeeper.SetLunaExchangeRate(
		input.Ctx, core.MicroUSDDenom, sdkmath.LegacyMustNewDecFromStr("0.0001"),
	)
	input.OracleKeeper.SetLunaExchangeRate(
		input.Ctx, core.MicroSDRDenom, sdkmath.LegacyMustNewDecFromStr("0.00008"),
	)
	input.OracleKeeper.SetLunaExchangeRate(
		input.Ctx, testEUTCOracleDenom, sdkmath.LegacyMustNewDecFromStr("0.02"),
	)
	input.MarketKeeper.SetLastOracleTallyTime(input.Ctx, input.Ctx.BlockTime().Unix())
	SeedCompleteTWAP(&input, map[string]sdkmath.LegacyDec{
		core.MicroUSDDenom:  sdkmath.LegacyMustNewDecFromStr("0.0001"),
		testEUTCOracleDenom: sdkmath.LegacyMustNewDecFromStr("0.02"),
	})

	pool := sdk.NewCoins(
		sdk.NewInt64Coin(core.MicroLunaDenom, 1_000_000_000),
		sdk.NewInt64Coin(core.MicroEURDenom, 1_000_000_000),
	)
	require.NoError(t, FundModuleAccount(input, markettypes.ModuleName, pool))

	offer := sdk.NewInt64Coin(core.MicroLunaDenom, core.MicroUnit)
	require.NoError(t, FundAccount(input, Addrs[0], sdk.NewCoins(offer)))
	eutcSupplyBefore := input.BankKeeper.GetSupply(input.Ctx, core.MicroEURDenom).Amount

	response, err := NewMsgServerImpl(input.MarketKeeper).Swap(
		sdk.WrapSDKContext(input.Ctx),
		markettypes.NewMsgSwap(Addrs[0], offer, core.MicroEURDenom),
	)
	require.NoError(t, err)
	require.Equal(t, core.MicroEURDenom, response.SwapCoin.Denom)
	require.True(t, response.SwapCoin.IsPositive())
	require.False(t, input.BankKeeper.GetSupply(input.Ctx, core.MicroEURDenom).Amount.GT(eutcSupplyBefore))

	_, err = NewMsgServerImpl(input.MarketKeeper).Swap(
		sdk.WrapSDKContext(input.Ctx),
		markettypes.NewMsgSwap(Addrs[0], offer, core.MicroUSDDenom),
	)
	require.ErrorIs(t, err, markettypes.ErrInvalidSwapPair)
}

func TestInitialLiquidityAcceptsAnyConfiguredQuoteReserve(t *testing.T) {
	input := CreateTestInput(t)
	input.MarketKeeper.SetMarketAssets([]MarketAssetConfig{testEUTCAsset()})

	lunc := sdk.NewCoins(sdk.NewInt64Coin(core.MicroLunaDenom, 1_000_000))
	require.NoError(t, FundModuleAccount(input, markettypes.ModuleName, lunc))
	require.False(t, input.MarketKeeper.hasInitialLiquidity(input.Ctx))

	eutc := sdk.NewCoins(sdk.NewInt64Coin(core.MicroEURDenom, 10_000))
	require.NoError(t, FundModuleAccount(input, markettypes.ModuleName, eutc))
	require.True(t, input.MarketKeeper.hasInitialLiquidity(input.Ctx))
}

func TestMarketAssetRegistryRejectsDuplicates(t *testing.T) {
	input := CreateTestInput(t)
	asset := testEUTCAsset()
	require.Panics(t, func() {
		input.MarketKeeper.SetMarketAssets([]MarketAssetConfig{asset, asset})
	})
}
