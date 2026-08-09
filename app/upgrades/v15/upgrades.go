package v15

import (
	"context"

	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/classic-terra/core/v4/app/keepers"
	"github.com/classic-terra/core/v4/app/upgrades"
	core "github.com/classic-terra/core/v4/types"
	markettypes "github.com/classic-terra/core/v4/x/market/types"
	oracletypes "github.com/classic-terra/core/v4/x/oracle/types"
	treasurytypes "github.com/classic-terra/core/v4/x/treasury/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

func CreateV15UpgradeHandler(
	mm *module.Manager,
	cfg module.Configurator,
	_ upgrades.BaseAppParamManager,
	k *keepers.AppKeepers,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)

		// Guarantee that tax redirection and epoch refill target a real module
		// account, including when the deterministic address already exists as a
		// base account on a pre-MM2 chain.
		k.MarketKeeper.EnsureMarketAccumulatorAccount(sdkCtx)

		// Preserve the legacy virtual-pool tuning while initializing every MM2
		// parameter to the approved no-mint values.
		marketParams := migrateMarketParams(k.MarketKeeper.GetParams(sdkCtx))
		k.MarketKeeper.SetParams(sdkCtx, marketParams)

		// Terra Classic uses uluna for governance deposits. Normalize both the
		// regular and expedited paths because SDK defaults use the generic
		// "stake" denom, which would otherwise block accelerated MM2 actions.
		govParams, err := k.GovKeeper.Params.Get(sdkCtx)
		if err != nil {
			return nil, err
		}
		govParams = migrateGovernanceParams(govParams)
		if err := k.GovKeeper.Params.Set(sdkCtx, govParams); err != nil {
			return nil, err
		}

		// Redirect 60% of tax proceeds into the next epoch's liquidity pool.
		treasuryParams := migrateTreasuryParams(k.TreasuryKeeper.GetParams(sdkCtx))
		k.TreasuryKeeper.SetParams(sdkCtx, treasuryParams)

		// Deploy inactive and anchor a complete collection epoch. The epoch
		// processor activates swaps only after both native pools are funded.
		k.MarketKeeper.InitializeMarketForUpgrade(sdkCtx)

		// Ensure every configured market price is an Oracle vote target. Existing
		// chains do not pick up DefaultParams changes automatically, so the upgrade
		// derives this list from the same registry used by swaps and TWAP.
		params := k.OracleKeeper.GetParams(sdkCtx)
		existingOracleDenoms := make(map[string]bool, len(params.Whitelist))
		for _, d := range params.Whitelist {
			existingOracleDenoms[d.Name] = true
		}
		var addedOracleDenoms []string
		for _, asset := range k.MarketKeeper.MarketAssets() {
			if existingOracleDenoms[asset.OracleDenom] {
				continue
			}
			params.Whitelist = append(params.Whitelist, oracletypes.Denom{
				Name:     asset.OracleDenom,
				TobinTax: sdkmath.LegacyZeroDec(),
			})
			existingOracleDenoms[asset.OracleDenom] = true
			addedOracleDenoms = append(addedOracleDenoms, asset.OracleDenom)
		}
		if len(addedOracleDenoms) > 0 {
			k.OracleKeeper.SetParams(sdkCtx, params)
			for _, denom := range addedOracleDenoms {
				// Set immediately so it becomes a vote target without waiting a full period.
				k.OracleKeeper.SetTobinTax(sdkCtx, denom, sdkmath.LegacyZeroDec())
			}
		}

		return mm.RunMigrations(ctx, cfg, fromVM)
	}
}

func migrateMarketParams(current markettypes.Params) markettypes.Params {
	migrated := markettypes.DefaultParams()
	if !current.BasePool.IsNil() {
		migrated.BasePool = current.BasePool
	}
	if current.PoolRecoveryPeriod != 0 {
		migrated.PoolRecoveryPeriod = current.PoolRecoveryPeriod
	}
	return migrated
}

func migrateTreasuryParams(current treasurytypes.Params) treasurytypes.Params {
	current.TaxRedirectRate = treasurytypes.DefaultTaxRedirectRate
	return current
}

func migrateGovernanceParams(current govv1.Params) govv1.Params {
	normalizeDepositDenom := func(coins []sdk.Coin) {
		for i := range coins {
			if coins[i].Denom == sdk.DefaultBondDenom {
				coins[i].Denom = core.MicroLunaDenom
			}
		}
	}

	normalizeDepositDenom(current.MinDeposit)
	normalizeDepositDenom(current.ExpeditedMinDeposit)
	return current
}
