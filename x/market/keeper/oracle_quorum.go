package keeper

import (
	"fmt"
	"sort"

	storetypes "cosmossdk.io/store/types"
	"github.com/classic-terra/core/v4/x/market/types"
	oracletypes "github.com/classic-terra/core/v4/x/oracle/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// IsOracleHalted reports whether the quorum guard is the reason Market is off.
func (k Keeper) IsOracleHalted(ctx sdk.Context) bool {
	value := ctx.KVStore(k.storeKey).Get(types.OracleHaltedKey)
	return len(value) == 1 && value[0] == storedBoolTrue
}

// SetOracleHalted persists the quorum guard state.
func (k Keeper) SetOracleHalted(ctx sdk.Context, halted bool) {
	value := byte(0)
	if halted {
		value = storedBoolTrue
	}
	ctx.KVStore(k.storeKey).Set(types.OracleHaltedKey, []byte{value})
}

// GetOracleMissedBlocks returns the consecutive sub-quorum block count.
func (k Keeper) GetOracleMissedBlocks(ctx sdk.Context, denom string) uint64 {
	value := ctx.KVStore(k.storeKey).Get(types.GetOracleMissedBlocksKey(denom))
	if len(value) != 8 {
		return 0
	}
	return sdk.BigEndianToUint64(value)
}

// SetOracleMissedBlocks persists or removes a per-denom quorum counter.
func (k Keeper) SetOracleMissedBlocks(ctx sdk.Context, denom string, blocks uint64) {
	store := ctx.KVStore(k.storeKey)
	key := types.GetOracleMissedBlocksKey(denom)
	if blocks == 0 {
		store.Delete(key)
		return
	}
	store.Set(key, sdk.Uint64ToBigEndian(blocks))
}

// GetOracleQuorumStates returns the stored counters in deterministic order.
func (k Keeper) GetOracleQuorumStates(ctx sdk.Context) []types.OracleQuorumState {
	store := ctx.KVStore(k.storeKey)
	iterator := storetypes.KVStorePrefixIterator(store, types.OracleMissedBlocksKey)
	defer iterator.Close()

	states := make([]types.OracleQuorumState, 0)
	for ; iterator.Valid(); iterator.Next() {
		denom := string(iterator.Key()[len(types.OracleMissedBlocksKey):])
		if len(iterator.Value()) != 8 {
			continue
		}
		states = append(states, types.OracleQuorumState{
			OracleDenom:             denom,
			ConsecutiveMissedBlocks: sdk.BigEndianToUint64(iterator.Value()),
		})
	}
	sort.Slice(states, func(i, j int) bool {
		return states[i].OracleDenom < states[j].OracleDenom
	})
	return states
}

// ClearOracleQuorumStates removes all per-denom quorum counters.
func (k Keeper) ClearOracleQuorumStates(ctx sdk.Context) {
	k.deleteByPrefix(ctx, types.OracleMissedBlocksKey)
}

func hasOracleQuorum(votePower, totalPower int64) bool {
	if votePower < 0 || totalPower <= 0 || votePower > totalPower {
		return false
	}
	// "Below 50%" is unhealthy, so exactly 50% remains healthy. Division is
	// avoided to preserve exact integer consensus semantics.
	return votePower >= totalPower-votePower
}

func addMissedBlocks(current, period uint64) uint64 {
	if current >= types.OracleQuorumMissedBlockLimit {
		return types.OracleQuorumMissedBlockLimit
	}
	if period >= types.OracleQuorumMissedBlockLimit-current {
		return types.OracleQuorumMissedBlockLimit
	}
	return current + period
}

func (k Keeper) updateOracleQuorum(ctx sdk.Context, votePeriod uint64, votePowers []oracletypes.DenomVotePower) {
	if votePeriod == 0 {
		return
	}

	observed := make(map[string]oracletypes.DenomVotePower, len(votePowers))
	for _, observation := range votePowers {
		observed[observation.Denom] = observation
	}

	allHealthy := true
	var haltCause oracletypes.DenomVotePower
	var haltMissedBlocks uint64
	for _, denom := range k.requiredMarketOracleDenoms() {
		observation, found := observed[denom]
		if !found {
			observation = oracletypes.DenomVotePower{Denom: denom}
		}

		if hasOracleQuorum(observation.VotePower, observation.TotalPower) {
			k.SetOracleMissedBlocks(ctx, denom, 0)
			continue
		}

		allHealthy = false
		missedBlocks := addMissedBlocks(k.GetOracleMissedBlocks(ctx, denom), votePeriod)
		k.SetOracleMissedBlocks(ctx, denom, missedBlocks)
		if missedBlocks >= types.OracleQuorumMissedBlockLimit && haltCause.Denom == "" {
			haltCause = observation
			haltMissedBlocks = missedBlocks
		}
	}

	if haltCause.Denom != "" && !k.IsOracleHalted(ctx) {
		k.SetOracleHalted(ctx, true)
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventOracleHalt,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(types.AttributeKeyOracleDenom, haltCause.Denom),
			sdk.NewAttribute(types.AttributeKeyVotePower, fmt.Sprintf("%d", haltCause.VotePower)),
			sdk.NewAttribute(types.AttributeKeyTotalPower, fmt.Sprintf("%d", haltCause.TotalPower)),
			sdk.NewAttribute(types.AttributeKeyMissedBlocks, fmt.Sprintf("%d", haltMissedBlocks)),
			sdk.NewAttribute(types.AttributeKeyEnabled, "false"),
			sdk.NewAttribute(types.AttributeKeyHeight, fmt.Sprintf("%d", ctx.BlockHeight())),
		))
		return
	}

	if allHealthy && k.IsOracleHalted(ctx) {
		k.SetOracleHalted(ctx, false)
		enabled := k.IsMarketEnabled(ctx)
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventOracleRecovery,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(types.AttributeKeyEnabled, fmt.Sprintf("%t", enabled)),
			sdk.NewAttribute(types.AttributeKeyHeight, fmt.Sprintf("%d", ctx.BlockHeight())),
		))
	}
}
