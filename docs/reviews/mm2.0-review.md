# Market Module 2.0 — Code Review

**Branch:** `mm-implementation` · **Reviewed at:** `c5bf7edb`
**Scope:** `x/market` (swap, safeguards, epoch), `x/tax` reverse-charge handler, oracle/treasury integration.

This review is split into **4 self-contained parts** so each can be posted as a separate PR comment and discussed independently. Each item ends with a *Question for the team* to make it easy to respond.

---

## Part 1/4 — Architecture assessment (context for everything below)

The single most important property of MM2.0 is that **it is no longer an algorithmic mint/burn stablecoin mechanism** — it is a bounded, tax-funded liquidity facility. This is what makes re-enabling LUNC↔USTC swaps defensible, so I want to confirm it is intentional before discussing individual findings.

**Evidence in code:**

1. **The market module cannot mint.** `app/modules.go:124` grants `markettypes.ModuleName` **`Burner` only** (no `Minter`). `AccumulatorModuleName` has no permissions (`app/modules.go:125`). The swap path (`x/market/keeper/msg_server.go`) never calls `MintCoins` — it pays out from a pre-funded module balance (`SendCoinsFromModuleToAccount`, `msg_server.go:195`) and fails with `ErrInsufficientLiquidity` (`msg_server.go:182`) when the pool is empty. The `MintCoins` method in `x/market/types/expected_keepers.go:24` appears vestigial (no production caller).

2. **Liquidity is a closed, non-inflationary loop:**
   ```
   burn-tax on transfers
     └─ TaxRedirectRate (default 0.6)                        treasury/types/params.go:52
          └─→ market_accumulator module account              x/tax/keeper/tax_split.go:25-31,122
                └─ every epoch (30d) ProcessEpochIfDue:       x/market/keeper/keeper.go:144-211
                     1) burn leftover pool balance
                     2) move accumulator → market pool
                          └─ swaps drain the finite pool (rate-limited by daily cap)
   ```
   No new LUNC is ever created; swaps redistribute already-collected tax proceeds. The reflexive "price down → more mint → more supply → price down" loop that drove the 2022 death spiral is structurally removed, not merely dampened.

3. **Four safeguards gate every swap** (all covered by `x/market/keeper/safeguards_test.go`, 12 tests): oracle freshness (`MaxOracleAgeSeconds`, default 75s), TWAP deviation (`MaxTwapDeviation`, default 10%), daily cap (`DailyCapFactor`, default 10%/day), min stability spread (default 2%), plus the meta-USD oracle rate that prices USTC at its real market value rather than the broken "1 USTC = $1" assumption.

**Verdict:** the direction is sound. Hyperinflation-by-emission is impossible by construction. Residual risk shifts to (a) oracle correctness, (b) safeguard behavior during bootstrap, and (c) economic defaults — covered in Parts 3–4.

> **Question for the team:** Can you confirm the intended model is "finite, tax-funded pool, no algorithmic minting", and that initial swap liquidity is seeded **into the accumulator** (not the market account directly)? See Part 3-B for why the distinction matters at upgrade height.

---

## Part 2/4 — Status of the three tracked findings

### Finding #1 — Allowed swap denoms · *Design / governance (not an active prod bug)*
- **Where:** `x/market/keeper/keeper.go:27-89`, gate at `msg_server.go:65-69`.
- **Observation:** `allowedSwapDenoms` is an **in-memory `map[string]bool`**, hard-defaulted to `{uusd}` in `NewKeeper`. It is **not** a governance param / genesis field / KV entry. `SetAllowedSwapDenoms` (`keeper.go:79`) **replaces** the map (`k.allowedSwapDenoms = m`); because the keeper is copied by value into the msg server and tax wrapper (`x/tax/modules/market/market_module.go:49-50`), calling it after wiring is a silent no-op on the paths that matter. Confirmed it is **only ever called in tests**; production runs the `{uusd}` default. Map is read-only after construction, so no data race / non-determinism in production.
- **Why it matters:** (a) governance cannot adjust the set without a coordinated binary upgrade; if two validators ever run binaries with different defaults they disagree on swap validity → consensus fork. (b) The public setter is a footgun for a future upgrade handler.
- **Suggested action:** either promote to a real governance param (genesis + migration), or make the setter mutate in place and drop it from the public surface.

> **Question for the team:** Is keeping the allowed set code-fixed (non-governable) a deliberate safety choice, or should it become a param so it can be tuned/extended without a chain upgrade?

### Finding #2 — Reverse-charge handler: unchecked type assertion → panic · *Medium*
- **Where:** `x/tax/keeper/keeper.go:161` — `if !ctx.Value(types.ContextKeyTaxReverseCharge).(bool)`.
- **Observation:** unchecked `.(bool)`. If the context key is absent (`nil`), this **panics**. Your own test documents this (`x/tax/handlers/market_msg_server_test.go:42-46,132`: "the value MUST be set explicitly (otherwise the handler panics)"). Normal txs (ante `custom/auth/ante/fee.go:94`) and wasm (`custom/wasm/keeper/handler_plugin.go:35`) always set it, so it is not reachable via ordinary user txs — but any non-ante execution context (e.g. a gov-executed `MsgSwap`, or a future internal caller) hits it.
- **Note (positive):** the reverse-charge *logic* is correct — ante-charge and reverse-charge are mutually exclusive (`fee.go:83-87`), so there is no double-tax and no evasion, and `DeductTax` runs in the same message context so a failed swap rolls the tax back atomically.
- **Suggested fix:**
  ```go
  func (k Keeper) IsReverseCharge(ctx sdk.Context, emit bool) bool {
      v, ok := ctx.Value(types.ContextKeyTaxReverseCharge).(bool)
      if !ok || !v {
          if emit { /* existing no-reverse-charge event */ }
          return false
      }
      return true
  }
  ```
- **Minor:** `msg.OfferCoin = netOfferCoin[0]` (`x/tax/handlers/market_msg_server.go:50,72`) mutates the incoming message in place (contained, but a smell).

> **Question for the team:** Any objection to hardening `IsReverseCharge` to treat a missing/invalid context value as `false` instead of relying on every caller to pre-set it?

### Finding #3 — TWAP encoding · *Resolved — acknowledgement only*
- **Was:** hand-rolled binary format (`height(8B) + len(4B) + price`) with manual offset parsing and silent `break`/skip on malformed data (original impl, commit `9dc72a8d`).
- **Now:** a proper protobuf message `PriceSnapshots { repeated PriceSnapshot }` (`proto/terra/market/v1beta1/market.proto:70-86`), marshalled via `cdc.MustMarshal`/`Unmarshal` with graceful corruption handling (`x/market/keeper/keeper.go:235-280`). This fully addresses the original concern. 👍
- **Optional polish (non-blocking):** (a) `ComputeTWAP` is a simple arithmetic mean, not truly time-weighted (`keeper.go:289`, comment acknowledges it) — fine at regular tally cadence, biased if cadence ever varies. (b) `TwapLookbackWindow` is a gov param stored as one blob per denom; a very large value would make each tally re-marshal a large slice for every oracle denom.

> **Question for the team:** Fine to leave the simple-mean TWAP as-is for launch, or do you want true time-weighting before enabling swaps?

---

## Part 3/4 — Additional findings

### A. Safeguards "fail open" during the bootstrap window · *Medium*
- **Where:** `x/market/keeper/msg_server.go:78` (freshness skipped when `lastTallyTime == 0`); `msg_server.go:113-129` (`checkTWAPDeviation` returns "allow" when there is no TWAP data **and on every price-read/compute error**).
- **Why it matters:** immediately after the upgrade — before the first oracle tally sets `lastTallyTime`, and before ~`TwapLookbackWindow` blocks of TWAP accumulate — swaps execute with **neither** freshness **nor** deviation protection (only spread + daily cap + liquidity remain). This window coincides with the moment the pool is freshly funded, i.e. the most attractive time to attack.
- **Suggested action:** make freshness fail-closed once initialized; consider a governance "swaps enabled" switch that flips only after a warmup period so the guards are populated.

> **Question for the team:** Is the fail-open bootstrap intentional, and if so how is the warmup window intended to be protected operationally?

### B. Epoch processing runs on the very first block · *Low-Med (deployment footgun)*
- **Where:** `x/market/keeper/keeper.go:145-150`. With `last == 0` the early-return guard is `false`, so `ProcessEpochIfDue` runs a full epoch on the first `EndBlock` — **burning the market account balance** before seeding baselines.
- **Why it matters:** if initial liquidity is placed **directly in the market account** at upgrade, it is burned on block 1. Liquidity must be seeded into the **accumulator** so the first-block epoch moves it into the pool. This is not documented in the v15 upgrade handler.
- **Suggested action:** document the seeding procedure, and/or guard the first-run epoch so it does not burn on a cold start.

> **Question for the team:** Where does the initial pool liquidity come from at the v15 upgrade — accumulator or a direct market-account funding?

### C. Default swap-fee burn is 0% · *Info / policy*
- **Where:** `x/market/types/params.go:43-44` (`DefaultSwapFeeBurnRate = 0`, `DefaultSwapFeeCommunityRate = 0`); applied by `app/upgrades/v15/upgrades.go:32-33`.
- **Observation:** with these defaults, **100% of the swap spread is routed to the oracle reward pool; none is burned**. On-chain "burning" then comes only from the per-epoch burn of leftover pool balance and the separate burn-tax — not from swap spread.
- **Why it matters:** many stakeholders associate swaps with burning LUNC/USTC; the actual default behavior differs. Worth stating explicitly in the governance/upgrade notes.

> **Question for the team:** Is 0% burn / 100%-to-oracle the intended launch policy for the swap spread?

### D. Daily cap is relative to the epoch-start baseline for the whole epoch · *Low-Med*
- **Where:** baseline set once per epoch in `ProcessEpochIfDue` (`keeper.go:202-207`); `CheckAndUpdateDailyCapForSwap` reads it (`keeper.go:429-442`); only daily *usage* resets intra-epoch (`keeper.go:385-394`).
- **Observation:** `dailyCap = DailyCapFactor × baseline` stays fixed for 30 days even as the pool drains. With defaults this means the pool can be fully drained in ~`1/DailyCapFactor` ≈ **10 days** of sustained one-directional pressure; late in a drained epoch the daily cap exceeds remaining balance, so the binding constraint degrades to the liquidity check.
- **Why it matters:** the rate-limit is a drain-horizon, not a floor on remaining liquidity. May be exactly intended — just confirming.

> **Question for the team:** Is the ~10-day full-drain horizon the intended design, or should the cap track the current balance rather than the epoch-start baseline?

### E. No trader slippage / min-receive protection · *Low*
- **Where:** `MsgSwap` / `MsgSwapSend` carry only `{Trader/From/To, OfferCoin, AskDenom}` — no `minReceive`.
- **Why it matters:** a trader has no protection against spread/pool-state movement between submission and execution. Partly mitigated by min-spread + TWAP, but non-standard for an AMM-style swap.

> **Question for the team:** Is an explicit min-receive field out of scope for this release?

### F. Mixed block-based vs time-based windows · *Low*
- **Where:** daily-cap reset is block-based (`core.BlocksPerDay = 14400`, `keeper.go:390`); oracle freshness is time-based (`MaxOracleAgeSeconds`, `msg_server.go:80`).
- **Why it matters:** under sustained block-time drift the "daily" cap window and the freshness window diverge from each other. Minor, but worth a comment.

### G. Spec is stale · *Doc*
- **Where:** `x/market/spec/01_concepts.md:73-79` still describes the old algorithmic model ("Burn offered coins … Mint `ask - fee` coins … Send newly minted coins"), which contradicts the current no-mint implementation.
- **Why it matters:** an auditor reading the spec would conclude the module still mints. Please update to reflect the finite-pool model.

---

## Part 4/4 — Economic & operational risks + recommendations

These are not code bugs; they are the places where the safety of MM2.0 now actually lives, and where I'd want sign-off before enabling swaps on mainnet.

1. **Oracle correctness is now safety-critical.** With minting removed, the dominant risk is mispricing. The meta-USD conversion (`x/market/keeper/swap.go:152-165`) divides by `usdPerUSTC`; a wrong or manipulated USTC/USD rate lets the pool be drained at bad rates (bounded by daily cap + liquidity, but a direct loss to the community-funded pool). **Recommend a dedicated economic/oracle audit of this path** — it deserves more than a code review.

2. **The pool is community property.** Sustained arbitrage that always drains the cheap side transfers value from taxpayers (all holders, via redirected tax) to arbitrageurs. If the oracle is accurate this is just efficient market-making; if not, it is a slow leak. The daily cap limits only the *rate* of that leak.

3. **The upgrade moment is the riskiest window** (Part 3-A + 3-B): guards fail open before warmup, and the first-block epoch can burn misplaced liquidity. Recommend an explicit, documented enablement runbook (seed accumulator → let guards warm up → enable swaps via governance switch).

4. **Governance cannot tune the allowed set** (Finding #1) — inflexible for something positioned as a managed stabilization tool.

**Overall:** architecturally MM2.0 turns a "bomb" into a "metered tap" — the right and serious step forward. It is not "enable and forget": launch safety rests on (i) oracle correctness, (ii) hardened bootstrap-window behavior, and (iii) the economic defaults in Part 3. I'd gate mainnet enablement on an economic audit of the oracle/meta-USD math plus fixes for Part 2 #2 and Part 3-A/B.

---

*Prepared as a code review of branch `mm-implementation` @ `c5bf7edb`. Line references are to that revision.*
