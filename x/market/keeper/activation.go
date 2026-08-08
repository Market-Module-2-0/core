package keeper

import (
	"fmt"

	core "github.com/classic-terra/core/v4/types"
	"github.com/classic-terra/core/v4/x/market/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const storedBoolTrue byte = 1

// IsMarketEnabled reports whether swap transactions may execute. The
// activation/governance state and the Oracle safety halt are independent so
// Oracle recovery can never override another reason for disabling Market.
func (k Keeper) IsMarketEnabled(ctx sdk.Context) bool {
	return k.IsMarketActivationEnabled(ctx) && !k.IsOracleHalted(ctx)
}

// IsMarketActivationEnabled reports the persisted base state without applying
// transient safety guards. A missing key is disabled so upgrades fail closed.
func (k Keeper) IsMarketActivationEnabled(ctx sdk.Context) bool {
	value := ctx.KVStore(k.storeKey).Get(types.MarketEnabledKey)
	return len(value) == 1 && value[0] == storedBoolTrue
}

// SetMarketEnabled persists the activation/governance base state.
func (k Keeper) SetMarketEnabled(ctx sdk.Context, enabled bool) {
	value := byte(0)
	if enabled {
		value = storedBoolTrue
	}
	ctx.KVStore(k.storeKey).Set(types.MarketEnabledKey, []byte{value})
}

// IsInitialActivationPending reports whether the first post-upgrade
// activation is waiting for a fully collected liquidity epoch.
func (k Keeper) IsInitialActivationPending(ctx sdk.Context) bool {
	value := ctx.KVStore(k.storeKey).Get(types.InitialActivationPendingKey)
	return len(value) == 1 && value[0] == storedBoolTrue
}

// SetInitialActivationPending persists the first-activation state.
func (k Keeper) SetInitialActivationPending(ctx sdk.Context, pending bool) {
	value := byte(0)
	if pending {
		value = storedBoolTrue
	}
	ctx.KVStore(k.storeKey).Set(types.InitialActivationPendingKey, []byte{value})
}

// InitializeMarketForUpgrade starts the first collection epoch exactly once.
// Re-running the migration does not reset the epoch anchor or disable a market
// that has already been initialized by a previous run.
func (k Keeper) InitializeMarketForUpgrade(ctx sdk.Context) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has(types.OracleHaltedKey) {
		k.SetOracleHalted(ctx, false)
	}
	if store.Has(types.MarketEnabledKey) {
		return
	}

	k.SetMarketEnabled(ctx, false)
	k.SetInitialActivationPending(ctx, true)
	k.SetLastEpochHeight(ctx, ctx.BlockHeight())
}

func (k Keeper) hasInitialLiquidity(ctx sdk.Context) bool {
	marketAddr := k.AccountKeeper.GetModuleAddress(types.ModuleName)
	lunc := k.BankKeeper.GetBalance(ctx, marketAddr, core.MicroLunaDenom)
	if !lunc.IsPositive() {
		return false
	}

	// The first activation requires LUNC and at least one configured quote
	// reserve. Additional configured assets become usable only once their own
	// physical reserve exists; they do not block the whole Market module.
	for _, asset := range k.MarketAssets() {
		if k.BankKeeper.GetBalance(ctx, marketAddr, asset.BankDenom).IsPositive() {
			return true
		}
	}
	return false
}

func (k Keeper) activateAfterInitialEpoch(ctx sdk.Context) {
	if !k.IsInitialActivationPending(ctx) || k.IsOracleHalted(ctx) || !k.hasInitialLiquidity(ctx) {
		return
	}

	k.SetMarketEnabled(ctx, true)
	k.SetInitialActivationPending(ctx, false)
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventActivation,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(types.AttributeKeyEnabled, "true"),
			sdk.NewAttribute(types.AttributeKeyHeight, fmt.Sprintf("%d", ctx.BlockHeight())),
		),
	)
}
