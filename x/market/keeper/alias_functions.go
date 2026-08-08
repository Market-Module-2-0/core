package keeper

import (
	"fmt"

	"github.com/classic-terra/core/v4/x/market/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// GetMarketAccount returns market ModuleAccount
func (k Keeper) GetMarketAccount(ctx sdk.Context) authtypes.ModuleAccountI {
	return k.AccountKeeper.GetModuleAccount(ctx, types.ModuleName)
}

// EnsureMarketAccumulatorAccount returns a valid module account for the MM2
// accumulator. Existing balances are stored by address in x/bank, so converting
// a base account at the deterministic module address preserves all funds.
func (k Keeper) EnsureMarketAccumulatorAccount(ctx sdk.Context) authtypes.ModuleAccountI {
	addr := k.AccountKeeper.GetModuleAddress(types.AccumulatorModuleName)
	if addr == nil {
		panic(fmt.Sprintf("%s module account has not been registered", types.AccumulatorModuleName))
	}

	account := k.AccountKeeper.GetAccount(ctx, addr)
	if account == nil {
		return k.AccountKeeper.GetModuleAccount(ctx, types.AccumulatorModuleName)
	}

	if moduleAccount, ok := account.(authtypes.ModuleAccountI); ok {
		if moduleAccount.GetName() != types.AccumulatorModuleName {
			panic(fmt.Sprintf("module account at %s has unexpected name %s", addr, moduleAccount.GetName()))
		}
		return moduleAccount
	}

	baseAccount := authtypes.NewBaseAccount(
		addr,
		nil,
		account.GetAccountNumber(),
		account.GetSequence(),
	)
	moduleAccount := authtypes.NewModuleAccount(baseAccount, types.AccumulatorModuleName)
	k.AccountKeeper.SetModuleAccount(ctx, moduleAccount)

	return moduleAccount
}
