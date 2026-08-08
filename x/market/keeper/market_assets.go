package keeper

import (
	"fmt"
	"sort"

	"cosmossdk.io/math"
	core "github.com/classic-terra/core/v4/types"
	oracletypes "github.com/classic-terra/core/v4/x/oracle/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MarketAssetPriceSource describes how an Oracle entry is converted into the
// legacy "asset units per LUNC" rate consumed by the Market swap calculation.
type MarketAssetPriceSource uint8

const (
	// MarketAssetPriceLunaRate means OracleDenom already stores asset units per
	// LUNC. It keeps legacy test and internal-denom support available.
	MarketAssetPriceLunaRate MarketAssetPriceSource = iota
	// MarketAssetPriceUSD means OracleDenom stores USD per one unit of the
	// market asset. The rate is combined with uusd (USD per LUNC).
	MarketAssetPriceUSD
)

// MarketAssetConfig is the single definition used by pair validation, price
// resolution, TWAP collection and future per-asset quorum checks. Adding an
// asset to this consensus configuration requires a coordinated chain upgrade;
// it does not activate any additional asset in the current binary.
type MarketAssetConfig struct {
	BankDenom   string
	OracleDenom string
	PriceSource MarketAssetPriceSource
}

func defaultMarketAssets() []MarketAssetConfig {
	return []MarketAssetConfig{{
		BankDenom:   core.MicroUSDDenom,
		OracleDenom: oracletypes.MetaUSDDenom,
		PriceSource: MarketAssetPriceUSD,
	}}
}

// SetMarketAssets replaces the in-memory consensus asset registry. Production
// uses the deterministic defaults compiled into the binary; this setter exists
// for application wiring and tests of future assets.
func (k *Keeper) SetMarketAssets(assets []MarketAssetConfig) {
	ordered := append([]MarketAssetConfig(nil), assets...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].BankDenom < ordered[j].BankDenom
	})

	byDenom := make(map[string]MarketAssetConfig, len(ordered))
	for _, asset := range ordered {
		if asset.BankDenom == "" || asset.BankDenom == core.MicroLunaDenom {
			panic(fmt.Sprintf("invalid market asset bank denom %q", asset.BankDenom))
		}
		if asset.OracleDenom == "" {
			panic(fmt.Sprintf("missing oracle denom for market asset %s", asset.BankDenom))
		}
		if asset.PriceSource != MarketAssetPriceLunaRate && asset.PriceSource != MarketAssetPriceUSD {
			panic(fmt.Sprintf("invalid price source for market asset %s", asset.BankDenom))
		}
		if _, exists := byDenom[asset.BankDenom]; exists {
			panic(fmt.Sprintf("duplicate market asset %s", asset.BankDenom))
		}
		byDenom[asset.BankDenom] = asset
	}

	k.marketAssets = ordered
	k.marketAssetsByDenom = byDenom
}

// MarketAssets returns a deterministic defensive copy of the asset registry.
func (k Keeper) MarketAssets() []MarketAssetConfig {
	return append([]MarketAssetConfig(nil), k.marketAssets...)
}

func (k Keeper) MarketAsset(denom string) (MarketAssetConfig, bool) {
	asset, found := k.marketAssetsByDenom[denom]
	return asset, found
}

func (k Keeper) marketAssetForPair(offerDenom, askDenom string) (MarketAssetConfig, bool) {
	if offerDenom == core.MicroLunaDenom {
		return k.MarketAsset(askDenom)
	}
	if askDenom == core.MicroLunaDenom {
		return k.MarketAsset(offerDenom)
	}
	return MarketAssetConfig{}, false
}

// oracleDenomsForAsset returns every Oracle input required to price an asset.
// Direct USD assets require both USD/LUNC and USD/asset; legacy assets already
// provide their complete LUNC rate in one entry.
func (k Keeper) oracleDenomsForAsset(asset MarketAssetConfig) []string {
	if asset.PriceSource == MarketAssetPriceUSD && asset.OracleDenom != core.MicroUSDDenom {
		return []string{core.MicroUSDDenom, asset.OracleDenom}
	}
	return []string{asset.OracleDenom}
}

func (k Keeper) validateMarketAssetOracle(ctx sdk.Context, asset MarketAssetConfig) error {
	for _, denom := range k.oracleDenomsForAsset(asset) {
		rate, err := k.OracleKeeper.GetLunaExchangeRate(ctx, denom)
		if err != nil || !rate.IsPositive() {
			return fmt.Errorf("missing positive oracle rate for %s", denom)
		}
	}
	return nil
}

// marketExchangeRate returns asset units per LUNC, matching the legacy Market
// calculation while allowing every depegged asset to carry its own USD price.
func (k Keeper) marketExchangeRate(ctx sdk.Context, denom string) (math.LegacyDec, error) {
	asset, configured := k.MarketAsset(denom)
	if !configured || asset.PriceSource == MarketAssetPriceLunaRate {
		return k.OracleKeeper.GetLunaExchangeRate(ctx, denom)
	}

	lunaUSD, err := k.OracleKeeper.GetLunaExchangeRate(ctx, core.MicroUSDDenom)
	if err != nil || !lunaUSD.IsPositive() {
		return math.LegacyZeroDec(), fmt.Errorf("missing positive USD/LUNC oracle rate")
	}
	assetUSD, err := k.OracleKeeper.GetLunaExchangeRate(ctx, asset.OracleDenom)
	if err != nil || !assetUSD.IsPositive() {
		return math.LegacyZeroDec(), fmt.Errorf("missing positive USD/%s oracle rate", asset.BankDenom)
	}

	return lunaUSD.Quo(assetUSD), nil
}

func (k Keeper) trackedMarketOracleDenoms() []string {
	denoms := map[string]struct{}{
		core.MicroSDRDenom: {}, // adaptive liquidity input
	}
	for _, denom := range k.requiredMarketOracleDenoms() {
		denoms[denom] = struct{}{}
	}

	ordered := make([]string, 0, len(denoms))
	for denom := range denoms {
		ordered = append(ordered, denom)
	}
	sort.Strings(ordered)
	return ordered
}

// requiredMarketOracleDenoms returns only the inputs needed to execute swaps.
// The SDR rate used by adaptive liquidity is deliberately excluded: a missing
// SDR rate already makes epoch processing retry safely, while GAP-002 concerns
// the prices of the assets exchanged by Market.
func (k Keeper) requiredMarketOracleDenoms() []string {
	denoms := make(map[string]struct{})
	for _, asset := range k.marketAssets {
		for _, denom := range k.oracleDenomsForAsset(asset) {
			denoms[denom] = struct{}{}
		}
	}

	ordered := make([]string, 0, len(denoms))
	for denom := range denoms {
		ordered = append(ordered, denom)
	}
	sort.Strings(ordered)
	return ordered
}
