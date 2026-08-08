package keeper

import (
	"fmt"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	core "github.com/classic-terra/core/v4/types"
	"github.com/classic-terra/core/v4/x/market/types"
	oracletypes "github.com/classic-terra/core/v4/x/oracle/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

// Keeper of the market store
type Keeper struct {
	storeKey   storetypes.StoreKey
	cdc        codec.BinaryCodec
	paramSpace paramstypes.Subspace

	AccountKeeper types.AccountKeeper
	BankKeeper    types.BankKeeper
	OracleKeeper  types.OracleKeeper
	DistrKeeper   types.DistributionKeeper

	// marketAssets is a deterministic consensus registry. The map is lookup-only;
	// consensus iteration always uses the sorted slice.
	marketAssets        []MarketAssetConfig
	marketAssetsByDenom map[string]MarketAssetConfig
}

// NewKeeper constructs a new keeper for oracle
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	paramstore paramstypes.Subspace,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
	oracleKeeper types.OracleKeeper,
	distrKeeper types.DistributionKeeper,
) Keeper {
	// ensure market module account is set
	if addr := accountKeeper.GetModuleAddress(types.ModuleName); addr == nil {
		panic(fmt.Sprintf("%s module account has not been set", types.ModuleName))
	}

	// set KeyTable if it has not already been set
	if !paramstore.HasKeyTable() {
		paramstore = paramstore.WithKeyTable(types.ParamKeyTable())
	}

	keeper := Keeper{
		cdc:           cdc,
		storeKey:      storeKey,
		paramSpace:    paramstore,
		AccountKeeper: accountKeeper,
		BankKeeper:    bankKeeper,
		OracleKeeper:  oracleKeeper,
		DistrKeeper:   distrKeeper,
	}
	keeper.SetMarketAssets(defaultMarketAssets())
	return keeper
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// GetTerraPoolDelta returns the gap between the TerraPool and the TerraBasePool
func (k Keeper) GetTerraPoolDelta(ctx sdk.Context) math.LegacyDec {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.TerraPoolDeltaKey)
	if bz == nil {
		return math.LegacyZeroDec()
	}

	dp := sdk.DecProto{}
	k.cdc.MustUnmarshal(bz, &dp)
	return dp.Dec
}

// SetTerraPoolDelta updates TerraPoolDelta which is gap between the TerraPool and the BasePool
func (k Keeper) SetTerraPoolDelta(ctx sdk.Context, delta math.LegacyDec) {
	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshal(&sdk.DecProto{Dec: delta})
	store.Set(types.TerraPoolDeltaKey, bz)
}

// ReplenishPools replenishes each pool(Terra,Luna) to BasePool
func (k Keeper) ReplenishPools(ctx sdk.Context) {
	poolDelta := k.GetTerraPoolDelta(ctx)

	poolRecoveryPeriod := int64(k.PoolRecoveryPeriod(ctx))
	poolRegressionAmt := poolDelta.QuoInt64(poolRecoveryPeriod)

	// Replenish pools towards each base pool
	// regressionAmt cannot make delta zero
	poolDelta = poolDelta.Sub(poolRegressionAmt)

	k.SetTerraPoolDelta(ctx, poolDelta)
}

// -------- Epoch processing (burn + refill) --------

func (k Keeper) GetLastEpochHeight(ctx sdk.Context) int64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.EpochLastHeightKey)
	if bz == nil {
		return 0
	}
	return int64(sdk.BigEndianToUint64(bz))
}

func (k Keeper) SetLastEpochHeight(ctx sdk.Context, h int64) {
	store := ctx.KVStore(k.storeKey)
	bz := sdk.Uint64ToBigEndian(uint64(h))
	store.Set(types.EpochLastHeightKey, bz)
}

// ProcessEpochIfDue burns leftover pool balances and refills from the accumulator module account
// when an epoch boundary is reached.
func (k Keeper) ProcessEpochIfDue(ctx sdk.Context) {
	last := k.GetLastEpochHeight(ctx)
	now := ctx.BlockHeight()
	epochLen := k.EpochLengthBlocks(ctx)
	if last != 0 && uint64(now-last) < epochLen {
		return
	}
	// Do not consume the first activation boundary while Oracle quorum is
	// halted. Keeping the original epoch anchor lets the same completed
	// collection epoch be processed as soon as quorum recovers.
	if k.IsInitialActivationPending(ctx) && k.IsOracleHalted(ctx) {
		return
	}

	marketAddr := k.AccountKeeper.GetModuleAddress(types.ModuleName)
	accumAddr := k.AccountKeeper.GetModuleAddress(types.AccumulatorModuleName)
	balances := k.BankKeeper.SpendableCoins(ctx, marketAddr)
	accumBalances := k.BankKeeper.SpendableCoins(ctx, accumAddr)

	// Calculate the next virtual-pool settings before mutating balances. If the
	// required oracle input is unavailable, leave the entire epoch untouched and
	// retry on the next block.
	adaptive, err := k.prepareAdaptiveLiquidityParams(ctx, balances, accumBalances)
	if err != nil {
		k.Logger(ctx).Error("market adaptive liquidity calculation failed", "err", err)
		return
	}

	// Burn all balances held by the market module account
	if !balances.Empty() {
		if err := k.BankKeeper.BurnCoins(ctx, types.ModuleName, balances); err != nil {
			k.Logger(ctx).Error("market epoch burn failed", "err", err)
			return
		}
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventEpochBurn,
				sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
				sdk.NewAttribute(types.AttributeKeyFromModule, types.ModuleName),
				sdk.NewAttribute(types.AttributeKeyAmount, balances.String()),
				sdk.NewAttribute(types.AttributeKeyHeight, fmt.Sprintf("%d", now)),
			),
		)
	}

	// Move all funds from accumulator to market module account
	if !accumBalances.Empty() {
		if err := k.BankKeeper.SendCoinsFromModuleToModule(ctx, types.AccumulatorModuleName, types.ModuleName, accumBalances); err != nil {
			k.Logger(ctx).Error("market epoch refill failed", "err", err)
			return
		}
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventEpochRefill,
				sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
				sdk.NewAttribute(types.AttributeKeyFromModule, types.AccumulatorModuleName),
				sdk.NewAttribute(types.AttributeKeyToModule, types.ModuleName),
				sdk.NewAttribute(types.AttributeKeyAmount, accumBalances.String()),
				sdk.NewAttribute(types.AttributeKeyHeight, fmt.Sprintf("%d", now)),
			),
		)
	}

	k.applyAdaptiveLiquidityParams(ctx, adaptive)
	k.SetLastEpochHeight(ctx, now)

	// Baselines and usage belong to the old physical pool and must not leak into
	// the new epoch.
	k.clearDailyCapBaselines(ctx)
	k.ClearDailyCapUsage(ctx)

	// Set daily cap baseline to the pool balance after epoch refill
	// This baseline remains constant for the entire epoch (30 days)
	poolBalances := k.BankKeeper.SpendableCoins(ctx, marketAddr)
	for _, coin := range poolBalances {
		k.SetDailyCapBaseline(ctx, coin.Denom, coin.Amount)
	}

	// Initialize daily cap tracking for the new epoch
	k.SetDailyCapResetHeight(ctx, now)

	k.activateAfterInitialEpoch(ctx)
}

// -------- Oracle tally tracking --------

// GetLastOracleTallyTime returns the timestamp of the last oracle tally
func (k Keeper) GetLastOracleTallyTime(ctx sdk.Context) int64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.LastOracleTallyTimeKey)
	if bz == nil {
		return 0
	}
	return int64(sdk.BigEndianToUint64(bz))
}

// SetLastOracleTallyTime stores the timestamp of the last oracle tally
func (k Keeper) SetLastOracleTallyTime(ctx sdk.Context, timestamp int64) {
	store := ctx.KVStore(k.storeKey)
	bz := sdk.Uint64ToBigEndian(uint64(timestamp))
	store.Set(types.LastOracleTallyTimeKey, bz)
}

// -------- TWAP price tracking --------

// GetTWAPPrices returns the recent price snapshots for a denom
func (k Keeper) GetTWAPPrices(ctx sdk.Context, denom string) []types.PriceSnapshot {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.GetTWAPPriceKey(denom))
	if bz == nil {
		return []types.PriceSnapshot{}
	}

	var wrapped types.PriceSnapshots
	if err := k.cdc.Unmarshal(bz, &wrapped); err != nil {
		// Corrupted entry: surface the error in the logs rather than silently truncating,
		// and treat as empty so callers fall back to bootstrapping behavior.
		k.Logger(ctx).Error("failed to unmarshal TWAP snapshots", "denom", denom, "err", err)
		return []types.PriceSnapshot{}
	}
	if wrapped.Snapshots == nil {
		return []types.PriceSnapshot{}
	}
	return wrapped.Snapshots
}

// AddTWAPPrice adds a new price snapshot and prunes old ones. Pruning always
// retains the latest observation at or before the lookback boundary because it
// defines the price in effect at the beginning of the TWAP interval.
func (k Keeper) AddTWAPPrice(ctx sdk.Context, denom string, price math.LegacyDec) {
	if !price.IsPositive() {
		return
	}

	snapshots := k.GetTWAPPrices(ctx, denom)
	currentHeight := ctx.BlockHeight()
	lookback := k.TwapLookbackWindow(ctx)

	// A tally can only occur once at a given height. Replacing a duplicate keeps
	// the stored step function canonical if this method is called twice in tests
	// or application wiring.
	if len(snapshots) > 0 && snapshots[len(snapshots)-1].Height == currentHeight {
		snapshots[len(snapshots)-1].Price = price
	} else {
		if len(snapshots) > 0 && snapshots[len(snapshots)-1].Height > currentHeight {
			return
		}
		snapshots = append(snapshots, types.PriceSnapshot{Height: currentHeight, Price: price})
	}

	pruneFrom := 0
	const maxInt64AsUint = uint64(1<<63 - 1)
	if lookback <= maxInt64AsUint {
		windowStart := currentHeight - int64(lookback)
		predecessor := -1
		for i, snap := range snapshots {
			if snap.Height <= windowStart {
				predecessor = i
				continue
			}
			break
		}
		if predecessor >= 0 {
			pruneFrom = predecessor
		}
	}
	pruned := snapshots[pruneFrom:]

	// Encode and store as a single introspectable protobuf message.
	store := ctx.KVStore(k.storeKey)
	if len(pruned) == 0 {
		store.Delete(types.GetTWAPPriceKey(denom))
		return
	}
	bz := k.cdc.MustMarshal(&types.PriceSnapshots{Snapshots: pruned})
	store.Set(types.GetTWAPPriceKey(denom), bz)
}

// ComputeTWAP calculates the price weighted by the exact number of blocks for
// which each observation was effective over [currentHeight-lookback,
// currentHeight). A snapshot at or before the interval start is mandatory;
// otherwise the window is incomplete and swaps must remain closed.
func (k Keeper) ComputeTWAP(ctx sdk.Context, denom string) (math.LegacyDec, error) {
	snapshots := k.GetTWAPPrices(ctx, denom)
	if len(snapshots) == 0 {
		return math.LegacyZeroDec(), fmt.Errorf("%w for %s", types.ErrTWAPNotReady, denom)
	}

	lookback := k.TwapLookbackWindow(ctx)
	const maxInt64AsUint = uint64(1<<63 - 1)
	if lookback == 0 || lookback > maxInt64AsUint {
		return math.LegacyZeroDec(), fmt.Errorf("%w for %s: invalid window %d", types.ErrTWAPNotReady, denom, lookback)
	}

	windowStart := ctx.BlockHeight() - int64(lookback)
	activeIndex := -1
	for i, snap := range snapshots {
		if snap.Height <= windowStart {
			activeIndex = i
			continue
		}
		break
	}
	if activeIndex < 0 || !snapshots[activeIndex].Price.IsPositive() {
		return math.LegacyZeroDec(), fmt.Errorf(
			"%w for %s: history does not cover height %d",
			types.ErrTWAPNotReady,
			denom,
			windowStart,
		)
	}

	weightedSum := math.LegacyZeroDec()
	activePrice := snapshots[activeIndex].Price
	cursor := windowStart
	currentHeight := ctx.BlockHeight()
	for _, snap := range snapshots[activeIndex+1:] {
		if snap.Height >= currentHeight {
			break
		}
		if snap.Height <= cursor {
			activePrice = snap.Price
			continue
		}
		if !activePrice.IsPositive() || !snap.Price.IsPositive() {
			return math.LegacyZeroDec(), fmt.Errorf("%w for %s: non-positive observation", types.ErrTWAPNotReady, denom)
		}
		weightedSum = weightedSum.Add(activePrice.MulInt64(snap.Height - cursor))
		cursor = snap.Height
		activePrice = snap.Price
	}
	if cursor < currentHeight {
		weightedSum = weightedSum.Add(activePrice.MulInt64(currentHeight - cursor))
	}

	return weightedSum.QuoInt64(int64(lookback)), nil
}

// -------- Daily cap tracking --------

func (k Keeper) GetDailyCapResetHeight(ctx sdk.Context) int64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.DailyCapResetHeightKey)
	if bz == nil {
		return 0
	}
	return int64(sdk.BigEndianToUint64(bz))
}

func (k Keeper) SetDailyCapResetHeight(ctx sdk.Context, h int64) {
	store := ctx.KVStore(k.storeKey)
	bz := sdk.Uint64ToBigEndian(uint64(h))
	store.Set(types.DailyCapResetHeightKey, bz)
}

// GetDailyCapBaseline returns the baseline balance for a denom set at epoch change
func (k Keeper) GetDailyCapBaseline(ctx sdk.Context, denom string) math.Int {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.GetDailyCapBaselineKey(denom))
	if bz == nil {
		return math.ZeroInt()
	}

	var amount sdk.IntProto
	k.cdc.MustUnmarshal(bz, &amount)
	return amount.Int
}

// SetDailyCapBaseline stores the baseline balance for a denom (set at epoch change)
func (k Keeper) SetDailyCapBaseline(ctx sdk.Context, denom string, amount math.Int) {
	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshal(&sdk.IntProto{Int: amount})
	store.Set(types.GetDailyCapBaselineKey(denom), bz)
}

// GetDailyCapUsage returns the amount drained today for a denom
func (k Keeper) GetDailyCapUsage(ctx sdk.Context, denom string) math.Int {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.GetDailyCapUsageKey(denom))
	if bz == nil {
		return math.ZeroInt()
	}

	var amount sdk.IntProto
	k.cdc.MustUnmarshal(bz, &amount)
	return amount.Int
}

// SetDailyCapUsage stores the amount drained today for a denom
func (k Keeper) SetDailyCapUsage(ctx sdk.Context, denom string, amount math.Int) {
	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshal(&sdk.IntProto{Int: amount})
	store.Set(types.GetDailyCapUsageKey(denom), bz)
}

// deleteByPrefix removes every entry under a store prefix. Keys are collected
// before deletion because mutating the store during iteration is undefined.
func (k Keeper) deleteByPrefix(ctx sdk.Context, prefix []byte) {
	store := ctx.KVStore(k.storeKey)
	iterator := storetypes.KVStorePrefixIterator(store, prefix)

	var keys [][]byte
	for ; iterator.Valid(); iterator.Next() {
		key := append([]byte(nil), iterator.Key()...)
		keys = append(keys, key)
	}
	iterator.Close()

	for _, key := range keys {
		store.Delete(key)
	}
}

// ClearDailyCapUsage removes all per-denom daily usage counters.
func (k Keeper) ClearDailyCapUsage(ctx sdk.Context) {
	k.deleteByPrefix(ctx, types.DailyCapUsageKey)
}

// clearDailyCapBaselines removes all per-denom daily cap baselines
func (k Keeper) clearDailyCapBaselines(ctx sdk.Context) {
	k.deleteByPrefix(ctx, types.DailyCapBaselineKey)
}

// ResetDailyCapIfNeeded resets daily usage counters if a day has passed
func (k Keeper) ResetDailyCapIfNeeded(ctx sdk.Context) {
	lastReset := k.GetDailyCapResetHeight(ctx)
	currentHeight := ctx.BlockHeight()

	// Reset every 14,400 blocks (1 day at 3s/block)
	if lastReset == 0 || currentHeight-lastReset >= int64(core.BlocksPerDay) {
		k.ClearDailyCapUsage(ctx)
		k.SetDailyCapResetHeight(ctx, currentHeight)
	}
}

// AfterOracleTally is called by the oracle module after a tally completes.
// It records only inputs used by configured assets plus the SDR input needed by
// adaptive liquidity. The same deterministic list can drive GAP-002 quorum.
func (k Keeper) AfterOracleTally(ctx sdk.Context, votePeriod uint64, votePowers []oracletypes.DenomVotePower) {
	currentTime := ctx.BlockTime().Unix()
	k.SetLastOracleTallyTime(ctx, currentTime)

	for _, denom := range k.trackedMarketOracleDenoms() {
		price, err := k.OracleKeeper.GetLunaExchangeRate(ctx, denom)
		if err == nil && price.IsPositive() {
			k.AddTWAPPrice(ctx, denom, price)
		}
	}

	k.updateOracleQuorum(ctx, votePeriod, votePowers)
}

// CheckAndUpdateDailyCapForSwap checks if a proposed swap would exceed daily cap limits and updates usage
// Each day allows draining up to DailyCapFactor × baseline (e.g. 10% of 1M = 100k per day).
// askCoin is the gross amount leaving Market, including the swap fee.
// When you swap A→B, you drain B and add A. Adding A back reduces B's drainage counter.
//
// askCoin must be the gross amount leaving the pool for the ask denom, i.e. the
// receiver's payout plus the swap fee, since the fee is carved out of the payout
// and leaves the pool as well (burn / community pool / oracle).
func (k Keeper) CheckAndUpdateDailyCapForSwap(ctx sdk.Context, offerCoin sdk.Coin, askCoin sdk.Coin) error {
	k.ResetDailyCapIfNeeded(ctx)

	dailyCapFactor := k.DailyCapFactor(ctx)

	// Check the ask denom (what's being drained from the pool)
	askBaseline := k.GetDailyCapBaseline(ctx, askCoin.Denom)
	if askBaseline.IsZero() {
		// A denom funded outside an epoch boundary must still be capped.
		marketAddr := k.AccountKeeper.GetModuleAddress(types.ModuleName)
		askBaseline = k.BankKeeper.GetBalance(ctx, marketAddr, askCoin.Denom).Amount
		if askBaseline.IsZero() {
			return nil
		}
		k.SetDailyCapBaseline(ctx, askCoin.Denom, askBaseline)
	}

	// Calculate daily cap for this denom
	dailyCap := dailyCapFactor.MulInt(askBaseline).TruncateInt()

	// Get current daily usage (how much has been drained today)
	currentUsage := k.GetDailyCapUsage(ctx, askCoin.Denom)

	// When we offer the same denom that was previously drained, we reduce its usage
	// Example: Day 1 drain 80k LUNC, Day 1 add back 40k LUNC → net drainage = 40k
	if offerCoin.Denom == askCoin.Denom {
		// This shouldn't happen in normal swaps (can't swap LUNC for LUNC)
		return nil
	}

	// Check if the offer denom was previously drained - if so, this swap adds it back
	offerBaseline := k.GetDailyCapBaseline(ctx, offerCoin.Denom)
	if !offerBaseline.IsZero() {
		// Reduce the offer denom usage by the amount we're adding back via offer
		// This is the key insight: if we drained LUNC and now offer LUNC, we're undoing the drainage
		offerUsage := k.GetDailyCapUsage(ctx, offerCoin.Denom)
		if offerUsage.IsPositive() {
			// We're adding back a denom that was previously drained
			reduction := offerCoin.Amount
			if reduction.GT(offerUsage) {
				reduction = offerUsage
			}
			k.SetDailyCapUsage(ctx, offerCoin.Denom, offerUsage.Sub(reduction))
		}
	}

	// Calculate new usage after draining askCoin
	newUsage := currentUsage.Add(askCoin.Amount)

	// Check if new usage would exceed daily cap
	if newUsage.GT(dailyCap) {
		return types.ErrDailyCapExceeded
	}

	// Update usage for ask denom (only if check passed)
	k.SetDailyCapUsage(ctx, askCoin.Denom, newUsage)

	return nil
}
