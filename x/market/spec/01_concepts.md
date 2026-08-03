<!--
order: 1
-->

# Concepts

## Market Module 2.0 model (important)

> **This module no longer mints or burns to service swaps.** In the original design a
> swap burned the offered coin and **minted** the asked coin, which allowed unbounded
> LUNC issuance and enabled the 2022 death spiral. Market Module 2.0 instead pays swaps
> out of a **finite, pre-funded liquidity pool** held by the `market` module account,
> which is granted **`Burner` only (no `Minter`)**.
>
> - Swaps draw from the pool and fail with `ErrInsufficientLiquidity` when it is empty.
> - The pool is funded from redirected burn-tax proceeds (`TaxRedirectRate`), which
>   accumulate in the `market_accumulator` module account.
> - Once per epoch (`EpochLengthBlocks`, default 30 days) the module burns any leftover
>   pool balance and refills the pool from the accumulator. **Initial liquidity must be
>   seeded into the accumulator, not the market account** — see *Epoch burn & refill*.
> - Every swap is additionally gated by safeguards: allowed-pair check (uluna ↔ an
>   allowed denom), oracle freshness (`MaxOracleAgeSeconds`), TWAP deviation
>   (`MaxTwapDeviation`), a daily drain cap (`DailyCapFactor`), and the minimum spread.
>
> The constant-product / spread math below still governs how the uluna ↔ Terra price
> and spread are computed; only the settlement mechanism (pool transfer vs mint/burn)
> has changed.

## Swap Fees
Since Terra's price feed is derived from validator oracles, there is necessarily a delay between the on-chain reported price and the actual realtime price.

This difference is on the order of about 1 minute (our oracle VotePeriod is 30 seconds), which is negligible for nearly all practical transactions. However an attacker could take advantage of this lag and extract value out of the network through a front-running attack.

To defend against this, the Market module enforces the following swap fees

* a Tobin Tax (set at 0.25%) for spot-converting Terra<>Terra swaps

    To illustrate, assume that oracle reports that the Luna<>SDT exchange rate is 10, and for Luna<>KRT, 10,000. Sending in 1 SDT will get you 0.1 Luna, which is 1000 KRT. After applying the Tobin Tax, you'll end up with 997.5 KRT (0.25% of 1000 is 2.5), a better rate than any retail currency exchange and remittance.

* a minimum spread (set at 2%) for Terra<>Luna swaps

    Using the same exchange rates above, swapping 1 SDT will return 980 KRT worth of Luna (2% of 1000 is 20, taken as the swap fee). In the other direction, 1 Luna would give you 9.8 SDT (2% of 10 = 0.2), or 9800 KRT (2% of 10,000 = 200).

## Market Making Algorithm
Terra uses a Constant Product market-making algorithm to ensure liquidity for Terra<>Luna swaps.

With Constant Product, we define a value `CP` set to the size of the Terra pool multiplied by a set fiat value of Luna, and ensure our market-maker maintains it as invariant during any swaps through adjusting the spread.

> NOTE - Our implementation of Constant Product diverges from Uniswap's, as we use the fiat value of Luna instead of the size of the Luna pool. This nuance means changes in Luna's price don't affect the product, but rather the size of the Luna pool.

```
CP = TerraPool * LunaPool * LunaPrice / SDRPrice
```

For example, we'll start with equal pools of Terra and Luna, both worth 1000 SDR total. The size of the Terra pool is 1000 SDT, and assuming the price of Luna<>SDR is 0.5, the size of the Luna pool is 2000 Luna. A swap of 100 SDT for Luna would return around 90.91 SDR worth of Luna (≈ 181.82 Luna). The offer of 100 SDT is added to the Terra pool, and the 90.91 SDT worth of Luna are taken out of the Luna pool.

```
CP = 1000000 SDR
(1000 SDT) * (1000 SDR of Luna) = 1000000 SDR
(1100 SDT) * (909.0909... SDR of Luna) = 1000000 SDR
```

Of course, this specific example was meant to be more illustrative than realistic -- with much larger liquidity pools used in production, the magnitude of the spread is diminished.

The primary advantage of Constant-Product over Columbus-2 is that it offers “unbounded” liquidity, in the sense that swaps of arbitrary size can be serviced (albeit at prices that become increasingly unfavorable as trade size increases).

## Virtual Liquidity Pools

The market starts out with two liquidity pools of equal sizes, one representing Terra (all denominations) and another representing Luna, initialiazed by the parameter `BasePool`, which defines the initial size of the Terra and Luna liquidity pools.

In practice, rather than keeping track of the sizes of the two pools, the information is encoded in a number `delta`, which the blockchain stores as `TerraPoolDelta`, representing the deviation of the Terra pool from its base size in units µSDR.

The size of the Terra and Luna liquidity pools can be generated from  using the following formulas:

```
TerraPool = BasePool + delta
LunaPool * LunaPice / SDRPrice = (BasePool * BasePool) / TerraPool
LunaPool = (SDRPrice / LunaPrice) * (BasePool * BasePool) / TerraPool
```

At the end of each block, the market module will attempt to "replenish" the pools by decreasing the magnitude of  between the Terra and Luna pools. The rate at which the pools will be replenished toward equilibrium is set by the parameter `PoolRecoveryPeriod`, with lower periods meaning lower sensitivity to trades, meaning previous trades are more quickly forgotten and the market is able to offer more liquidity.

This mechanism ensures liquidity and acts as a sort of low-pass filter, allowing for the spread fee (which is a function of TerraPoolDelta) to drop back down when there is a change in demand, hence necessary change in supply which needs to be absorbed.

## Swap Procedure

1. Market module receives `MsgSwap`/`MsgSwapSend` and performs basic validation checks.

2. Enforce the safeguards: the pair must be uluna ↔ an allowed denom
   (`isAllowedSwapDenom`), the oracle meta rate must exist, and the oracle tally must be
   fresh (`MaxOracleAgeSeconds`).

3. Calculate `ask` and `spread` using `k.ComputeSwap()`.

4. Enforce the TWAP deviation guard on each non-uluna leg (`MaxTwapDeviation`).

5. Compute the spread fee `fee = spread * ask` and subtract it from the asked amount.

6. Update `TerraPoolDelta` with `k.ApplySwapToPool()`.

7. Transfer `OfferCoin` from the trader to the `market` module account with
   `SendCoinsFromAccountToModule()`. **The offered coin is not burned** — it stays in the
   pool as liquidity for the opposite direction.

8. Check that the pool holds enough of the ask denom to cover the payout plus the fee,
   and that the swap does not exceed the daily drain cap (`DailyCapFactor`). If either
   fails, the swap is rejected (`ErrInsufficientLiquidity` / `ErrDailyCapExceeded`).

9. Pay the asked amount to the receiver from the pool with `SendCoinsFromModuleToAccount()`.
   **No coins are minted.**

10. Split the spread fee per governance params (`SwapFeeBurnRate`, `SwapFeeCommunityRate`,
    remainder to the oracle reward pool).

11. Emit the `swap` event with the spread fee.

If the pool has insufficient liquidity for the ask denom, or the daily cap would be
exceeded, the swap transaction fails.

## Seigniorage
Market Module 2.0 does **not** produce seigniorage by minting Terra: swaps are settled
from the finite pool, so no new supply is created. Value flows are limited to (a) the
spread fee split (burn / community pool / oracle), and (b) the per-epoch burn of any
leftover pool balance before the pool is refilled from the accumulator. See the
*Epoch burn & refill* behavior in `ProcessEpochIfDue` and the Treasury module's
`TaxRedirectRate`, described [here](../../treasury/spec/README.md).
