# Market Module 2.0 No-Mint Validation Report

> Status: local validation campaign completed
>
> Campaign start date: July 16, 2026
>
> Last updated: August 9, 2026
>
> Language: English
>
> Final conclusion: **NO-GO for public community testing or mainnet in the current state**

## Part I — Tests Performed and Corrections

This part records the executed scenarios, their evidence, and the corrections made for issues revealed by testing. Part II is reserved for mechanisms added to improve MM2 compliance, safety, or maintainability beyond the direct correction of a failing test.

### 1. Executive Summary

This report documents the local validation of the Terra Classic Market Module 2.0 No-Mint implementation. Its purpose is to assess functional compliance, economic safety, and failure resistance before opening the module to possible community testing.

The campaign strictly distinguishes:

- failures in MM2 code;
- limitations or defects in the test environment;
- gaps between the approved proposal and the implementation;
- risks that can only be fully evaluated on a public testnet.

All existing Go test suites pass, together with the newly added No-Mint economic invariants. On the single-validator devnet, swaps correctly draw from the prefunded pool, fees follow the local test profile, caps are atomic, and epoch rotations burn residual balances without Market minting.

This functional success is not yet sufficient to open the module to community testing. The three critical defects identified by the campaign have been corrected and revalidated locally: the `UST` rate contract, `market_accumulator` initialization, and the v15 migration with inactive deployment followed by deferred first activation. Draft reviews are now open for both the core changes and the feeder correction, but neither change is part of a released version. The upgrade must also be repeated against a representative pre-v15 snapshot.

The compliance audit also identified the absence of adaptive `base_pool` and `pool_recovery_period` calculations. This mechanism is now implemented and locally revalidated, while the direct per-asset cap remains a strict safety layer. A persistent halt after 25 blocks below 50% Oracle voting power, controlled recovery, and a true 45-block TWAP are also implemented and covered by deterministic tests.

GAP-004 was ultimately reclassified. The existing governance brake already existed as `MinStabilitySpread = 100%`, the value historically used on Columbus-5. It closes swaps by producing a zero net output. The expedited governance route also has the required `0.667` threshold, but its minimum deposit still used the generic `stake` denom, which is unusable on Terra Classic. Custom genesis and the v15 migration now normalize both regular and expedited deposits to `uluna`. Closure, non-mutation, persistence, and reopening are tested.

The `E2E-001` infrastructure blocker is also resolved. The official harness now starts four native ARM64 validators, confirms their P2P connections, and produces blocks. The baseline scenario builds a complete TWAP history through three Oracle cycles, then successfully executes LUNC → USTC and USTC → LUNC swaps. A second scenario assigns unequal validator powers of 40%, 30%, 20%, and 10% and validates the complete GAP-002 lifecycle: exactly 50% remains active, three commit/reveal rounds at 40% sustain an uninterrupted sub-quorum interval beyond the 25-block limit and halt swaps with committed Market code 10, and restored quorum re-enables swaps after TWAP reconstruction. A third scenario validates expedited governance with the same powers: 60% does not pass the `0.667` threshold, while 70% closes Market at a 100% spread and later reopens it at 0.35%. The verdict nevertheless remains **NO-GO** until the draft changes are reviewed and integrated and the upgrade is tested on a representative snapshot.

### 2. Frozen References

| Item | Tested reference |
|---|---|
| MM2.0 No-Mint proposal | `Market-Module-2-0/proposal@e576826a8163d4aacb64be0709822399dd970d5f` |
| Terra Classic MM2 base | `c5bf7edb5628bb15a45d7c2a0744c74dd877ec14` plus the commits in draft core PR #3 |
| Local branch | `mm2-development` |
| Source branch | `upstream/feat/mm-implementation` |
| Core review | [Market-Module-2-0/core#3](https://github.com/Market-Module-2-0/core/pull/3), draft |
| StrathCole Oracle feeder | `89cd2983015f47aa6fe005ebc3eb35e24789ba1a` plus correction `508711dff4d26d45bf39c5d7d0a87692de25dba6` |
| Feeder review | [StrathCole/oracle-go#1](https://github.com/StrathCole/oracle-go/pull/1), draft |
| Node image | `terra-classic-devnet:mm2` (`441d70cc604d`) |
| Feeder image | baseline `ee5143babfc8`, local fix `952be0ca1363` |
| Multi-validator E2E image | `terra:debug`, built natively for `linux/arm64` |

### 3. Environment

| Component | Value |
|---|---|
| Machine | Apple Silicon ARM64 |
| Host system | macOS / Darwin 25.5.0 |
| Go | 1.24.7 darwin/arm64 |
| Docker Engine | 28.1.1 linux/arm64 |
| Docker Desktop | 4.41.2 |
| Chain ID | `mm2-local-1` |
| Node | healthy |
| Oracle feeder | healthy |
| Oracle voting period | every 5 heights |
| Local MM2 epoch | 100 blocks |

The node, its data, and its ports are isolated from any `terrad` installation on the host. Validator secrets are not included in this report.

### 4. Observed MM2 Parameters

| Parameter | Local value |
|---|---:|
| Minimum spread | 0.35% |
| Swap-fee burn | 50% |
| Community Pool | 0% |
| Remaining fee sent to Oracle | 50% |
| Maximum Oracle age | 75 seconds |
| TWAP window | 45 blocks |
| Maximum TWAP deviation | 10% |
| Strict daily cap | 10% of baseline |
| Adaptive calculation factor | 7% |
| Tax redirection to accumulator | 60% |

### 5. Summary Results

| ID | Area | Test | Status |
|---|---|---|---|
| BASE-001 | Build/tests | Market, Oracle, Tax, and Treasury modules without cache | PASS |
| BASE-002 | Regression | Full `go test -count=1 ./...` suite | PASS |
| UNIT-001 | Invariants | No-Mint and fee accounting in both swap directions | PASS |
| UNIT-002 | Epoch | Burn balances, then refill without monetary creation | PASS |
| FUZZ-001 | Robustness | Positive quote fuzzing, 118,368 executions | PASS |
| ENV-001 | Environment | Go execution in sandbox with global cache | ENVIRONMENT |
| ORA-001 | Live Oracle | Feeder healthy and connected to node | PASS |
| ORA-002 | Live Oracle | Prevotes and votes executed on-chain | PASS |
| ORA-003 | Live Oracle | `uusd` and `UST` rates available on-chain | PASS |
| ORA-004 | Oracle outage | Atomic rejection, rate removal, feeder recovery, and weighted quorum guard | PASS, including four-validator GAP-002 E2E |
| ENV-002 | Local profile | `usdr` rate required by the virtual pool | RESOLVED |
| INT-001 | Core ↔ feeder | `UST` meta-denom semantics | **RESOLVED LOCAL — DRAFT PR OPEN** |
| INT-002 | Tax ↔ Market | `market_accumulator` initialization | **RESOLVED LOCAL — DRAFT PR OPEN** |
| INT-003 | v15 upgrade | Migration and first activation of a pre-MM2 chain | **RESOLVED LOCAL — DRAFT PR OPEN** |
| TAX-001 | E2E taxation | 60% redirection to accumulator | PASS |
| EPOCH-001 | E2E epoch | Complete burn of previous pool and complete refill | PASS |
| SWAP-001 | E2E swap | LUNC → USTC, pool-funded output, and 50/50 fee split | PASS after INT-001 correction |
| SWAP-002 | E2E swap | USTC → LUNC, pool-funded output, and 50/50 fee split | PASS after INT-001 correction |
| SAFE-001 | Daily cap | Rejection above 10% and atomicity | PASS |
| SAFE-002 | Allowed pair | USTC → SDR rejection and atomicity | PASS |
| LOAD-001 | Load | 20 sequenced swaps and epoch-boundary behavior | PASS |
| RES-001 | Restart | Persistence of heights, balances, and Oracle rates | PASS |
| RES-002 | Export/import | Complete export and restart from a clean home | PASS — default export fixed locally |
| E2E-001 | Multi-validator | Official four-validator harness, Oracle, TWAP, and bidirectional swaps | **RESOLVED — TARGETED TEST PASS** |
| E2E-002 | Oracle quorum | Unequal powers, 50% boundary, 40% halt, and full recovery | **PASS — FOUR-VALIDATOR NETWORK** |
| E2E-003 | Governance | Expedited 0.667 threshold, 100% spread closure, atomic rejection, and reopening | **PASS — FOUR-VALIDATOR NETWORK** |
| IMP-001 to IMP-007 | Improvements | Adaptive liquidity, epoch hardening, asset registry, Oracle auto-halt, true TWAP, and governance brake | **IMPLEMENTED LOCAL — TESTS PASS** |
| GAP-002 | Oracle safety | Quorum auto-halt using persistent activation state | **RESOLVED LOCAL — MULTI-VALIDATOR E2E PASS** |
| GAP-003 | TWAP | Truly weighted average and safe bootstrap phase | **RESOLVED LOCAL — TESTS PASS** |
| GAP-004 | Governance | Closure and reopening through historical spread, expedited path, and persistence | **RESOLVED LOCAL — MULTI-VALIDATOR E2E PASS** |
| CORE-001 | Operations | `terrad export` without a module list | **RESOLVED LOCAL — TESTS PASS** |

`ENV-001` and `ENV-002` are not functional MM2 failures. `E2E-001` was an infrastructure defect and is now resolved. `CORE-001`, now corrected locally, belongs to general application operation rather than MM2 economic logic.

### 6. Baseline Evidence

#### BASE-001 — Directly Affected Modules

Command:

```bash
go test -count=1 ./x/market/... ./x/oracle/... ./x/tax/... ./x/treasury/...
```

Result: every package containing tests returned `ok`.

#### BASE-002 — Complete Repository

Command:

```bash
go test -count=1 ./...
```

Result: every package containing tests returned `ok`. No panic, compilation failure, or failing test was observed.

#### UNIT-001, UNIT-002, and FUZZ-001 — Added Invariants

Deterministic tests cover both swap directions, absence of minting, conservation of flows between trader and pool, burn accounting, the Oracle account, and epoch burn/refill. They pass without cache. A 30-second fuzz run over quote calculation executed 118,368 cases without a panic, negative value, or unexpected zero result.

#### TAX-001 — On-Chain Tax Distribution

A taxable transaction produced `1,000,000 uluna` and `500,000 uusd` in tax. Events and balances confirm the following distribution:

| Destination | `uluna` | `uusd` | Share |
|---|---:|---:|---:|
| Community Pool | 800 | 400 | 0.08% |
| Oracle | 39,200 | 19,600 | 3.92% |
| `market_accumulator` | 600,000 | 300,000 | **60%** |
| Burn | 360,000 | 180,000 | 36% |

Transaction: `432EAA6B05C98E2AD94616F17BB163768B5FE9E40BF2703476B48239C0AA10E9`, height 1558.

#### EPOCH-001 — Pool Rotation

At the 1600/1601 epoch boundary, the full accumulator balance moved to `market`: `9,999,600,000 uluna` and `99,800,000 uusd`. At the next 1700/1701 boundary, after both swaps, the residual pool of `9,818,973,524 uluna` and `100,794,495 uusd` was completely burned. The `uusd` supply decreased by exactly `100,794,495` in that block. The `uluna` supply simultaneously reflects the pool burn and normal Mint-module issuance; events separate the two flows.

#### SWAP-001 and SWAP-002 — Real Execution and Accounting

| Direction | Offer | Received | Fee | Burn | Oracle | Hash |
|---|---:|---:|---:|---:|---:|---|
| LUNC → USTC | `1,000,000 uluna` | `5,486 uusd` | `19 uusd` | `9 uusd` | `10 uusd` | `DC0CE1B0…F2201D` |
| USTC → LUNC | `1,000,000 uusd` | `180,990,784 uluna` | `635,692 uluna` | `317,846 uluna` | `317,846 uluna` | `D6B7DAF1…ED657B` |

In both cases, the offer was credited to the `market` account, the output and fees were debited from that same account, and Market emitted no mint event. The amounts are mechanically consistent with the Oracle rate received, but they were economically incorrect before the `INT-001` correction.

#### SAFE-001 and SAFE-002 — Atomic Rejections

A `2,000,000,000 uluna` swap quoted at `10,973,993 uusd` exceeded the 10% cap of a `99,800,000 uusd` reference pool. Transaction `42631C30…1AB3E6` was rejected with Market code 9 (`daily swap cap exceeded`). Pool balances remained exactly `9,999,600,000 uluna` and `99,800,000 uusd`.

An attempted `1,000,000 uusd` to `usdr` swap (`AA414446…D1C19`) was rejected with Market code 5 (`invalid swap pair; not allowed`). The pool also remained unchanged. In both cases, only Ante transaction fees were charged to the signer, as expected in the Cosmos SDK; no state produced by the message was retained.

#### ORA-004 — Feeder Outage and Recovery

The feeder was stopped for more than 75 seconds. The native Oracle removed rates that no longer met its voting threshold. Swap `F8E352D9…` at height 1971 was rejected with Market code 3 (`no price registered with oracle`), and the pool remained unchanged. The more specific `oracle price stale` error was not reached because native rate removal occurs before the MM2 freshness limit in this profile.

After restart, the feeder became healthy and `UST`, `uusd`, and `usdr` were voted on-chain again in approximately 20 seconds. This experiment predates the GAP-002 fix. It validates safe rejection and operational recovery, but by itself is not an E2E proof of the auto-halt. The deterministic coverage under `IMP-005` and the later four-validator `E2E-002` scenario now supply that proof.

#### LOAD-001 — Swap Series and Epoch Boundary

Twenty swap transactions were sent with valid account sequences. The first fifteen were included with code 0. The epoch boundary at height 4801 then burned the residual pool; the following five transactions were rejected with Market code 6 (`insufficient pool liquidity`). No partial pool debit was observed.

A truly simultaneous submission from one address first exposed client nonce management, not an MM2 defect. The campaign therefore does not claim a parallel-throughput benchmark. It validates repeated swaps, atomicity, and state changes concurrent with an epoch boundary.

#### RES-001 — Restarts

The node was restarted from height 2040 and returned healthy at height 2056. Balances, parameters, and Market state persisted; the feeder reconnected and resumed voting. Several later controlled stops around heights 13,858 to 14,063 confirmed the same behavior.

#### RES-002 and CORE-001 — Export/Import

A targeted export of `auth`, `bank`, `market`, `oracle`, `treasury`, and `tax` produced valid JSON containing every expected MM2 parameter. A full export also succeeded when explicitly given the 23 registered modules. The resulting genesis started at height 14,063 with one validator.

This genesis was mounted in a clean container without the devnet volume. With the local-only validator key temporarily copied into place, the imported chain finalized blocks 14,063 through 14,066. The copied key was then deleted.

**Initial finding.** The following default command failed:

```bash
terrad export --home /var/lib/terra --height -1
```

Error: `module crisis does not exist`. The `crisis` name appeared in begin-block, end-block, and init/export orderings even though the module is not registered in `appModules`. This general core inconsistency did not corrupt MM2 data, but it required the `--modules-to-export` workaround.

**Implemented solution.** The three obsolete `crisis` references were removed from these orderings. The module was not reintroduced because it is not part of the currently assembled application. A generic regression test now builds the application in memory and verifies that every name in the begin-block, end-block, initialization, and export orderings corresponds to a registered module.

**Result.** The corrected Docker image was rebuilt, then `terrad export` was executed without `--modules-to-export` against a real node. The command exited with code 0 and produced all 23 registered modules, including `market`, without advertising absent `crisis` state. The ordering-consistency test and the complete `app` package suite also pass. `CORE-001` is resolved locally and the workaround is no longer required.

#### E2E-001 — Official Multi-Validator Harness

**Initial status: BLOCKED — ENVIRONMENT**

**Current status: RESOLVED — TARGETED TEST PASS**

##### Initial Finding

The targeted official test `TestIntegrationTestSuite/TestMarketSwap` created four containers, but the chain remained at height 0. The causes accumulated: every P2P configuration included the node's own ID, validators were started sequentially while the first already waited for consensus, and the image forced `linux/amd64` on an Apple Silicon host. Under emulation, P2P connections failed with errors including:

```text
secret conn failed: failed to decrypt SecretConnection:
chacha20poly1305: message authentication failed
```

After consensus was restored, other scenario defects became visible: Oracle gas above the Ante limit, redundant feeder delegation, a parser incompatible with legacy module-account JSON, an Oracle salt that was too long, a voting helper capable of interpreting a CLI error as success, a two-second freshness window incompatible with four validators, and a swap attempted before the TWAP was complete.

##### Implemented Solution

- every node receives all persistent peers except itself;
- all four containers start before any consensus wait, after which each must see the other three peers and advance several blocks;
- the E2E image follows the target Docker architecture and selects the matching `wasmvm` library;
- Oracle transactions explicitly use a `1,000,000` gas limit, and broadcast or execution failures are fatal;
- the test uses the validator's implicit feeder, four-character salts, and a parser compatible with both legacy and protobuf account JSON;
- E2E freshness uses the 75-second production value;
- the scenario executes three complete prevote/vote cycles so the first observation truly covers the 45-block TWAP window before any swap;
- active reserves are funded immediately before the transactions, after any epoch rotations that may occur during bootstrap.

Regression tests verify self-peer exclusion, inclusion of all other peers, complete group startup before readiness checks, and both module-account JSON formats.

##### Result

On August 8, 2026, the targeted test passed in `138.61 s` on four `linux/arm64` validators. Every validator submitted three valid Oracle prevotes and three valid votes. Rates were tallied, the complete TWAP was accepted, and the following transactions were included with code 0:

- `1,000,000 uluna` to `uusd`, with the trader's LUNC balance decreasing and USTC balance increasing;
- `500,000 uusd` to `uluna`, with the trader's USTC balance decreasing and LUNC balance increasing.

After the harness adopted the 40% / 30% / 20% / 10% validator distribution for E2E-002, this bidirectional baseline was replayed on August 9, 2026. The scenario passed again in `128.45 s`, confirming that unequal voting power did not regress the healthy Oracle, TWAP, or two-way swap path.

`E2E-001` is therefore closed as an infrastructure defect. This original evidence covers consensus → Oracle → TWAP → Market with four equal-power validators. Governance closure is covered separately by E2E-003; the full E2E suite with IBC and state sync remains outside this baseline.

#### E2E-002 — Unequal-Power Oracle Quorum Halt and Recovery

**Status: PASS**

On August 9, 2026, `TestMarketOracleQuorumHaltRecovery` passed on the official four-validator ARM64 harness. Genesis assigned and on-chain staking queries confirmed the following bonded powers:

| Validator | Bonded stake | Voting-power share |
|---|---:|---:|
| 0 | `40,000,000,000 uluna` | 40% |
| 1 | `30,000,000,000 uluna` | 30% |
| 2 | `20,000,000,000 uluna` | 20% |
| 3 | `10,000,000,000 uluna` | 10% |

The scenario produced the following sequence on a live chain:

1. all four validators completed three Oracle prevote/vote cycles, built the 45-block TWAP, and executed a baseline LUNC → USTC swap;
2. validators 0 and 3 supplied exactly 50% of bonded power, after which the same swap remained executable;
3. validator 0 alone supplied 40% for three complete commit/reveal rounds; every intervening tally also remained below quorum, sustaining the outage beyond 25 blocks and causing the persistent counter to reach its cap;
4. a swap was accepted by CheckTx, committed in a block, and rejected by DeliverTx with codespace `market`, code `10`, and `market module is disabled`;
5. all validators resumed voting for three cycles, rebuilding uninterrupted Oracle/TWAP history, after which a new swap committed with code 0 and increased the trader's USTC balance.

The targeted suite passed in `413.35 s`; the scenario itself passed in `397.34 s`. This closes the outstanding multi-validator outage validation for GAP-002. It also proves the exact 50% boundary and recovery behavior with real weighted staking power rather than mocked keeper state.

#### E2E-003 — Multi-Validator Expedited Governance Brake

**Status: PASS**

On August 9, 2026, `TestMarketExpeditedGovernanceBrake` passed on the same four-validator ARM64 harness with bonded powers of 40%, 30%, 20%, and 10%. The test used fully funded expedited v1 proposals carrying legacy `market/MinStabilitySpread` parameter-change content and the corrected `uluna` expedited deposit.

The live-chain sequence was:

1. all validators built a complete Oracle/TWAP history and a baseline LUNC → USTC swap succeeded with the approved `0.0035` spread;
2. validators representing 60% voted YES and 40% voted NO on a proposal to set the spread to `1`; this did not pass the `0.667` expedited threshold, the proposal was converted to the regular path, and Market remained at `0.0035`;
3. because the SDK consumes expedited votes during conversion, all four validators cast a fresh regular vote at 40% YES and 60% NO; the proposal was rejected and Market again remained unchanged;
4. a new expedited proposal received 70% YES and 30% NO, passed, and changed the on-chain spread to `1`;
5. after an independent Oracle/TWAP rebuild, a swap committed with Market code 4 (`zero swap coin`), while trader and Market LUNC/USTC balances remained unchanged;
6. a second 70%/30% expedited proposal restored `0.0035`; after TWAP reconstruction, the swap committed with code 0 and increased the trader's USTC balance.

The first execution of the scenario revealed the SDK vote-consumption behavior during expedited-to-regular conversion. This was a test-harness assumption rather than an MM2 defect. The final scenario explicitly requires a new multi-validator vote after conversion, making the operational behavior reproducible and visible to reviewers.

The targeted suite passed in `494.51 s`; the scenario itself passed in `479.36 s`. This closes the outstanding multi-validator governance validation for GAP-004 and brackets the expedited threshold with real staking power: 60% fails while 70% passes.

### 7. Validation Matrix

| Area | Main requirement | Current level |
|---|---|---|
| Swap | LUNC ↔ USTC only | PASS, stable-to-stable rejected |
| No-Mint | no Market supply increase during a swap | PASS in unit and E2E tests |
| Fees | 0.35%, 50% burn, 50% Oracle | PASS with forced local parameters |
| Liquidity | no output larger than the pool | PASS |
| Atomicity | no message state retained after rejection | PASS |
| Taxation | 60% to accumulator | PASS; local migration covered |
| Epoch | burn residual balances, then refill | PASS with a 100-block local epoch |
| Oracle | real and recent USTC price | PASS after local feeder correction |
| TWAP | reject above 10% of a truly weighted 45-block TWAP | deterministic PASS; incomplete history rejected |
| Daily cap | maximum 10% and reset | E2E PASS for excess rejection |
| Quorum | halt below 50% power for 25 blocks | deterministic PASS and four-validator unequal-power E2E PASS, including 50% boundary, halt, and recovery |
| Governance | expedited closure at 0.667 and deferred activation | deterministic and four-validator E2E PASS; 60% fails and 70% closes/reopens |
| Adaptive liquidity | recalculate `base_pool` and PRP at epoch | local PASS; 7% adaptive factor |
| Resilience | restart and export/import | PASS, including default export |
| Upgrade | migrate pre-MM2 state | local PASS from state missing new keys; real snapshot pending |

### 8. Confirmed Issues and Corrections

#### INT-001 — Incompatible `UST` Rate Format

**Initial status: FAIL**

**Severity: critical / P0**

**Scope: integration between the MM2 core and the StrathCole Oracle feeder**

The MM2 core documents and uses the `UST` meta-denom as the USD price of one USTC. The tested feeder instead converted `USTC/USD` into a `USTC per LUNC` rate before submitting it. Both components were functional in isolation, but their data contracts were incompatible.

Values observed during testing:

| Data | Value |
|---|---:|
| LUNC/USD price exposed by feeder | `0.000059107250373` |
| USTC/USD price exposed by feeder | `0.0054963836173671` |
| On-chain `UST` rate | `0.010754254719151968` |
| Quote for 1 LUNC to USTC after spread | `0.005477 USTC` |
| Quote for 1 USTC to LUNC after spread | `181.287156 LUNC` |

The on-chain `UST` rate approximately matched:

```text
LUNC/USD ÷ USTC/USD = USTC per LUNC
```

The core then interpreted it as:

```text
USD per USTC
```

As a result, MM2 quotes did not match the LUNC/USTC market ratio. In the sample above, the LUNC-to-USTC quote was close to half the expected value, while the reverse quote was almost doubled.

Reproduction commands:

```bash
terrad query oracle exchange-rates --output json
terrad query market swap 1000000uluna uusd --output json
terrad query market swap 1000000uusd uluna --output json
```

**Impact:** no community testnet should use this core/feeder combination because swaps would be economically mispriced even though Oracle votes and transactions were technically valid.

##### Retest After the Local Feeder Correction

**Current status: RESOLVED LOCAL — [DRAFT PR OPEN](https://github.com/StrathCole/oracle-go/pull/1)**

The selected contract is the one already documented by the core: `UST` directly carries `USD per USTC`. The feeder's `mm2-ust-price` branch now treats this meta-denom as an exception and does not apply the historical `fiat per LUNC` conversion.

Values observed on August 5, 2026 after rebuilding the Oracle image:

| Data | Value |
|---|---:|
| LUNC/USD price exposed by feeder | `0.0000501052732435` |
| USTC/USD price exposed by feeder | `0.0049910125303164` |
| On-chain `UST` rate | `0.004991012530316400` |
| Quote for 1 LUNC to USTC after spread | `0.010003 USTC` |
| Quote for 1 USTC to LUNC after spread | `99.261887 LUNC` |

Two real transactions then confirmed both directions and fee accounting:

| Direction | Offer | Received | MM2 fee | Burn | Oracle | Height | Hash |
|---|---:|---:|---:|---:|---:|---:|---|
| LUNC → USTC | `1,000,000 uluna` | `9,981 uusd` | `35 uusd` | `17 uusd` | `18 uusd` | 394787 | `01652C80…59D5556F` |
| USTC → LUNC | `1,000,000 uusd` | `99,481,916 uluna` | `349,409 uluna` | `174,704 uluna` | `174,705 uluna` | 394797 | `B7A66F4D…DF05A78` |

Core deterministic tests and voter-component tests pass with the same definition. `INT-001` is isolated and corrected locally. It should only be considered closed for a community release after review, merge, and inclusion in a released feeder version.

#### ENV-002 — `usdr` Rate Missing from the Initial Local Profile

**Status: RESOLVED**

**Classification: test environment, not a core defect**

The first local whitelist contained only `uusd` and `UST`. The Market Module constant-product calculation still uses `usdr` as its internal unit, so every quote failed with `no price registered with oracle`.

The test profile was corrected by adding `usdr` to the whitelist and the official IMF source to the feeder. The rate was then voted on-chain, and quote queries reached the MM2 calculation.

#### INT-002 — `market_accumulator` Module Account Not Initialized at Genesis

**Initial status: FAIL**

**Severity: critical / P0**

**Scope: integration between Tax, Bank, Auth, and Market**

On a fresh chain, Market genesis explicitly initialized the `market` module account but not `market_accumulator`. A bank transfer to the accumulator's deterministic address could therefore create a regular `BaseAccount` at that address first. The tax post-handler would later attempt to transfer the tax share through a module-to-module transfer. The SDK detected that the existing address was not a `ModuleAccount`, panicked, and BaseApp recovered the panic by rejecting the transaction.

Local reproduction transaction:

```text
C297FA5A49B5E9D51C1D0A144ECCE5327467EB360A7185FA62FA0D730ECF092C
```

Observed result at height 310:

```text
code: 111222
raw_log: account is not a module account
```

The stack trace placed the failure in `tax/keeper.ProcessTaxSplits` during the transfer from `fee_collector` to `market_accumulator`. No amount was credited to the destination. The node remained healthy: this was a transaction-level panic recovered by BaseApp, not a process shutdown.

Confirmed code cause:

- `x/market/genesis.go` forced creation of only the `market` module account;
- the SDK lazily creates an absent module account but panics if a regular account already occupies its deterministic address;
- `market_accumulator` is a declared authorized destination, so its existence must be guaranteed before any user transaction.

**Impact:** a transaction sent to the accumulator address before initialization could fail and prevent the expected module account from being established. The MM2 tax and pool-funding paths must not depend on the ordering of a chain's first transactions.

##### Retest After the Local Core Correction

**Current status: RESOLVED LOCAL — [DRAFT PR OPEN](https://github.com/Market-Module-2-0/core/pull/3)**

The core now guarantees `market_accumulator` through two paths:

- `InitGenesis` explicitly creates the module account when absent;
- the v15 upgrade handler performs the same idempotent operation before migrations.

If the deterministic address already exists as a `BaseAccount`, it is converted into a `ModuleAccount`. Account number and sequence are preserved. Balances remain unchanged because the Bank module records them by address.

Two regression tests cover creation from absent state and conversion of a bank account holding `12,345 uusd`. The second calls initialization twice to verify idempotence. After conversion, the module-account name, account number, sequence, and balance are all preserved.

Executed validation:

```bash
go test -count=1 ./x/market/...
go test -count=1 ./app/upgrades/v15 ./x/tax/keeper ./x/treasury/...
go test -count=1 ./...
```

All core suites pass. `INT-002` should be considered closed for a community release after review and integration.

#### INT-003 — v15 Upgrade Handler Did Not Migrate Pre-MM2 State

**Initial status: FAIL**

**Severity: critical / P0**

**Scope: activation on an existing Terra Classic chain**

During the initial audit, `app/upgrades/v15/upgrades.go` limited swaps in memory to `uusd`, added the `UST` Oracle meta-denom when necessary, and ran module-declared migrations. Even with the `market_accumulator` guarantee added by `INT-002`, it did not write any of the new parameter keys:

- `EpochLengthBlocks`;
- `SwapFeeBurnRate` and `SwapFeeCommunityRate`;
- `MaxOracleAgeSeconds`;
- `TWAPLookbackWindow` and `MaxTWAPDeviation`;
- `DailyCapFactor`;
- `TaxRedirectRate` in Treasury.

In `x/params`, `Subspace.Get` panics when a key is missing. A chain created before MM2 does not have these keys. Every `EndBlock` calls `ProcessEpochIfDue`, which immediately reads `EpochLengthBlocks`. The risk was therefore not merely an incorrect default: the first block after upgrade could halt chain execution.

Even if those keys were present, the initial handler did not replace the 100% spread historically used to disable swaps on Columbus-5. The audited code defaults also differed from the proposal: 2% spread instead of 0.35%, 0% fee burn instead of 50%, and therefore 100% of the remaining fee sent to Oracle instead of 50%. No two-stage activation was implemented.

**Impact:** the mainnet upgrade path was not safely executable, and the functional devnet profile existed only because local genesis explicitly forced the intended values.

##### Retest After the Local Core Correction

**Current status: RESOLVED LOCAL — [DRAFT PR OPEN](https://github.com/Market-Module-2-0/core/pull/3)**

The v15 handler now performs an idempotent migration that:

- guarantees the `market_accumulator` module account;
- preserves legacy `BasePool` and `PoolRecoveryPeriod` values;
- replaces the historical spread with `0.35%`;
- initializes the 30-day epoch, 50% burn / 50% Oracle fee split, 75-second Oracle freshness, 45-block TWAP window, 10% maximum TWAP deviation, and 10% daily cap;
- initializes Treasury tax redirection at 60%;
- replaces the generic `stake` denom in regular or expedited governance deposits with `uluna` without changing amounts or thresholds;
- writes persistent Market-disabled state and anchors the first collection epoch at upgrade height;
- preserves the original state and height if the handler is replayed.

The epoch processor then activates swaps only after one complete epoch and after a successful accumulator transfer into Market reserves containing both `uluna` and `uusd`. Before that boundary, every swap returns `market module is disabled`. Activation state, initial waiting state, and the last epoch height are included in genesis export/import.

The added tests cover two levels:

1. an in-memory application where the seven new Market keys, the Treasury key, and activation keys are deleted to reproduce pre-MM2 state;
2. the height 100 → height 110 sequence, with a rejected swap during collection, funding of both reserves, automatic activation, and then a successful first LUNC → USTC swap.

Executed validation:

```bash
go test -count=1 ./app/upgrades/v15
go test -count=1 ./x/market/...
go test -count=1 ./...
```

The next operational validation must still run the binary against a representative pre-v15 snapshot, produce several post-upgrade blocks, and confirm the real tax flow over a simulated 30-day collection period. This does not reopen the locally closed deterministic defect, but it remains mandatory before a community release.

## Part II — Implemented Improvements

This part separates implemented improvements from gaps that remain open. An improvement receives an `IMP` identifier when it adds or strengthens a design mechanism, even when it originated from analysis of a `GAP`.

### 9. Implemented Improvements

| ID | Improvement | Result |
|---|---|---|
| IMP-001 | Adaptive recalculation of `base_pool` and PRP with a 7% factor | Implemented and tested |
| IMP-002 | Oracle prevalidation before epoch rotation to avoid predictable partial mutation | Implemented and tested |
| IMP-003 | Stronger direct cap: fees included, baselines cleared, and absolute 10% maximum enforced | Implemented and tested |
| IMP-004 | Generic registry connecting bank denom, Oracle price source, TWAP, and authorized LUNC pair | Implemented and tested; only USTC active in production |
| IMP-005 | Voting-power-weighted Oracle auto-halt after 25 blocks below 50%, persistence, and controlled recovery | Implemented and tested locally; unequal-power outage and recovery E2E pass on four nodes |
| IMP-006 | Duration-weighted 45-block TWAP and closed bootstrap without complete history | Implemented and tested locally |
| IMP-007 | Reuse of historical spread as governance brake, expedited deposit correction, and closure/reopening validation | Implemented and tested locally |

#### IMP-001 to IMP-003 — GAP-001 Resolution and Hardening

**Initial status: ABSENT**

**Current status: RESOLVED LOCAL — 7% ADAPTIVE / 10% STRICT CAP**

**Initial severity: high / P1**

##### Initial Finding

The proposal requires an epoch recalculation based on the new pool balance, LUNC supply, the burst factor, and two caps. The initial code burned the pool, transferred the accumulator, and refreshed the cap baseline, but left `BasePool` and `PoolRecoveryPeriod` static.

##### Implemented Solution

The epoch processor now applies the following model using the Market module's historical micro-SDR units:

```text
PRP = max(14,400, ceil(14,400 × LUNC supply after burn / 1T LUNC))
desired adaptive daily cap = SDR value of the new LUNC pool × F, where F = 0.07
raw base_pool = desired daily cap × PRP / (2 × 14,400)
base_pool = min(raw base_pool, 0.00010 × SDR value of supply, 5,000,000 SDR)
```

The calculation runs before any epoch mutation. It uses the accumulator's LUNC balance, the supply that will remain after the old pool is burned, and a positive, recent LUNC/SDR Oracle rate. If that rate is missing or stale, the burn, refill, and epoch height remain unchanged; the same epoch boundary is retried on the next block. After application, the virtual pool delta resets to equilibrium so a reduced `base_pool` does not retain an incompatible prior imbalance.

The direct daily cap is not removed. It remains the strict safety layer for each physical asset, while `base_pool` and PRP progressively regulate the spread curve and its recovery. Direct-cap calculation now includes the complete output from the Market account—user payout plus fee—creates a baseline when an asset is funded outside an epoch boundary, and removes old baselines and usage during rotation.

The added tests validate:

- the default 7% factor with 600 million LUNC worth 24,000 SDR and a 6.5 trillion LUNC supply: a 93,600-block PRP and a `base_pool` of 5,460 SDR;
- contraction to a 1 trillion LUNC supply: a 14,400-block PRP and a `base_pool` of 840 SDR;
- the supply-value proportional cap and the absolute 5 million SDR cap;
- deferral without mutation when an epoch lacks a fresh Oracle price, followed by success after price publication;
- parameter application during first activation and execution of the first swap;
- fee inclusion in the direct cap, old-epoch counter clearing, and rejection of any configuration above the absolute 10% maximum.

The proposal contains a drafting inconsistency: it explicitly defines `F = 0.07`, states “at most 10%,” then uses `F = 0.1` in its example. The local implementation uses the explicit 7% default to size `base_pool`. The 10% value remains only the strict daily per-asset cap. A regression test enforces this separation so changing the strict cap cannot silently alter the adaptive factor. The proposal example should be clarified before mainnet.

#### IMP-004 — Generalized Market Assets

##### Initial Finding

The first MM2 code handled `uusd` and the `UST` meta-denom through special conditions distributed across pair validation, rate calculation, TWAP, activation, and upgrade logic. Adding another asset in the same manner would duplicate checks and multiply the paths requiring future fixes.

##### Implemented Solution

A shared `MarketAssetConfig` definition now connects:

- the bank denom physically held by the pool;
- the Oracle denom carrying the market price;
- the price-conversion mode into the historical “asset units per LUNC” format.

Pair validation, price resolution, TWAP collection, initial-liquidity conditions, Oracle target insertion during upgrade, and the GAP-002 quorum check all consume the same deterministic registry. No additional USTC-specific condition was added for the auto-halt.

Production configuration still contains only `uluna ↔ uusd`. Tests use a fictional EUTC meta-denom to demonstrate, without activating it, that the same code:

- calculates both directions from independent USD prices;
- executes a complete swap from physical reserves without increasing supply;
- collects the TWAP inputs required by the configured asset;
- accepts that reserve for first activation;
- continues to reject stable-to-stable swaps and assets absent from the registry.

Adding EUTC or another real asset remains a chain-level decision. It will require a coordinated upgrade, a final Oracle denom, compatible feeders, sufficient quorum, physical reserves, and a dedicated test campaign. Generalization removes code duplication but does not bypass these economic and operational prerequisites.

#### IMP-005 — GAP-002 Resolution: Voting-Power-Weighted Oracle Auto-Halt

**Initial status: ABSENT**

**Current status: RESOLVED LOCAL — MULTI-VALIDATOR OUTAGE E2E PASS**

**Initial severity: high / P1**

##### Initial Finding

The persistent activation state introduced by INT-003 could reject swaps, but no voting-power data crossed the Oracle hook and no counter measured 25 consecutive blocks below 50%. Native removal of an invalid rate already caused swaps to fail, but did not implement the proposal's duration, persistent halt, or recovery semantics.

##### Implemented Solution

After each tally, Oracle now sends Market a deterministic observation for every voting target: denom, voting power that submitted a positive price, and total active-validator power. This measurement is produced on-chain before the native tally removes denoms that miss its threshold. It therefore depends on neither the official feeder nor the StrathCole feeder, nor on the number of feeder processes running; only validator voting power matters.

For the production LUNC/USTC pair, the asset registry simultaneously requires quorum for both price inputs actually used:

- `uusd`, USD/LUNC price;
- `UST`, USD/USTC price.

After each Oracle period, a persistent per-denom counter increases by the period's block count when voting power is strictly below 50%. Exactly 50% is sufficient. A healthy period immediately resets that denom's counter. When any counter reaches 25 blocks, Market enters persistent `oracle_halted` state and rejects swaps. The counter is capped at 25 to avoid unnecessary growth.

The halt clears only when one tally shows sufficient quorum for every registry-required input. Quorum recovery only reopens swaps when the base activation state is also open; it cannot bypass first post-upgrade activation or an independent governance brake. Conversely, an epoch boundary cannot activate Market while `oracle_halted` is true. If this occurs at the first boundary, the epoch is neither consumed nor moved: it is retried after Oracle recovery without imposing another 30-day wait. Halt and recovery transitions emit dedicated events containing height, failing denom, and observed power.

Exported state contains the halt reason and per-denom counters in deterministic order. Import therefore restores an outage and its elapsed duration exactly. The v15 migration explicitly initializes the new state without resetting an already migrated installation.

The deterministic tests cover:

- five successive 5-block tallies at 49%, with halt only on block 25;
- the exact 50% boundary and reset of an earlier counter;
- a denom completely absent from the Oracle result;
- continued halt when only one of the two prices recovers;
- recovery when every required price is healthy;
- two-way interaction with deferred first activation, retry of its epoch boundary, and an independent disable state;
- export/import persistence;
- a generic fictional EUTC configuration without production activation;
- real transmission of weighted power from the Oracle EndBlocker into the Market hook with three equal-power validators.

The network scenario extends this coverage with validators holding 40%, 30%, 20%, and 10% of bonded stake. It proves that exactly 50% remains healthy, three 40% commit/reveal rounds sustain sub-quorum state beyond the 25-block limit and produce a committed code-10 rejection, and restored full quorum clears the guard and permits swaps after rebuilding the TWAP. The deterministic keeper tests separately prove the exact block-25 transition. This correction therefore closes both the deterministic code gap and the previously pending unequal-power outage E2E validation.

#### IMP-006 — GAP-003 Resolution: True TWAP and Closed Bootstrap

**Initial status: PARTIAL**

**Current status: RESOLVED LOCAL — TESTS PASS**

**Initial severity: high / P1**

##### Initial Finding

Every Oracle observation was already stored with its height, but `ComputeTWAP` ignored that information and calculated an arithmetic mean. A price valid for 40 blocks therefore had the same weight as a price observed for 5 blocks. With irregular or missed tallies, the result did not represent the price actually in effect during the window.

For example, with a price of `1` for 40 blocks followed by `2` for 5 blocks:

```text
previous arithmetic mean = (1 + 2) / 2 = 1.5
true TWAP                = (1 × 40 + 2 × 5) / 45 = 1.111…
```

The second defect was more direct: when no history existed, the swap handler deliberately skipped the TWAP check and allowed the transaction. A newly activated Market, a restoration without history, or a period with insufficient observations therefore had less protection than normal operation.

##### Implemented Solution

The calculation now treats observations as a step function over the exact interval:

```text
[current height − 45, current height)
```

For every segment, price is multiplied by the exact number of blocks for which it remained active. The sum is divided by 45. An observation published at the current height therefore has no retroactive weight and begins contributing on the next block.

Storage pruning also retains the last price at or before the boundary. This observation is required to know the price active at the start of the window, even when the prior tally is older because of irregular voting.

The presence of one snapshot is no longer sufficient. Every Oracle input required by the pair must have an observation at or before the window boundary. For LUNC/USTC, the rule applies separately to `uusd` and `UST`. If one input covers only 44 blocks, the swap returns Market error 11, `complete TWAP history is not available`, before any pool or balance mutation.

The first post-upgrade activation normally has a complete epoch in which to build this history. In every other case, behavior is closed by default: after a restoration without snapshots, swaps wait for 45 blocks of observations before resuming. A normal restart from the same storage preserves snapshots and does not cause this delay.

##### Regression Tests

Added or updated tests verify:

- exact result `50 / 45` for irregular durations of 40 blocks at `1` and 5 blocks at `2`;
- retention of the snapshot before the window boundary during pruning;
- rejection with only 44 blocks of history and acceptance at block 45;
- atomic swap rejection when `UST` covers 45 blocks but `uusd` covers only 44;
- success of the same swap one block later, when both windows are complete;
- deviations above and below 10%;
- native swap, `SwapSend`, Wasm, No-Mint, daily-cap, first-activation, and generic-asset paths.

#### IMP-007 — GAP-004 Resolution: Existing Governance Brake and Usable Expedited Path

**Initial status: PARTIAL, THEN RECLASSIFIED**

**Current status: RESOLVED LOCAL — TESTS PASS**

**Initial severity: high / P1**

##### Initial Finding

The initial analysis looked for a dedicated boolean or command allowing governance to stop Market. A new switch is unnecessary. The existing `MinStabilitySpread` parameter is already registered in the `market` subspace and can be modified through parameter-change governance. Historical application forks set it to `1`, or 100%, specifically to neutralize swaps. A query of Columbus-5 state on August 8, 2026 also returned this value.

At 100%, the calculated fee equals the gross output. The trader's net output becomes zero, and the message fails with `zero swap coin`. Restoring the approved spread to `0.0035` reopens the swap path, provided base activation, Oracle, TWAP, cap, and liquidity conditions are also valid. This brake is independent from Oracle auto-halt and first activation; none of these mechanisms can reopen another.

The review did reveal a configuration defect in expedited governance. The `0.667` threshold and shorter period existed, but the SDK-derived expedited minimum deposit used `stake`, while custom genesis converted only the regular deposit to `uluna`. An expedited proposal could therefore not be funded with Terra Classic's native token.

##### Implemented Solution

No additional Market halt state was introduced. The solution keeps the historical mechanism to avoid competing sources of truth:

- `MinStabilitySpread = 1` closes swaps;
- `MinStabilitySpread = 0.0035` restores the approved MM2 spread;
- zero-output, fee, and liquidity validation now occurs before any pool-delta or balance mutation;
- governance genesis uses `uluna` for regular and expedited deposits;
- the v15 migration converts a possible SDK `stake` denom to `uluna` on both paths while preserving amounts, periods, and the `0.667` expedited threshold.

##### Regression Tests and Network Evidence

Keeper tests successively close and reopen:

- a LUNC-to-USTC swap;
- a USTC-to-LUNC swap;
- a LUNC-to-USTC `SwapSend`.

While closed, trader, receiver, Market account, and `TerraPoolDelta` remain strictly unchanged. The same message succeeds after returning to `0.0035`. A genesis export/import also preserves the closed value of `1`.

A full-application integration test then submits two real expedited proposals, funds each with the required `uluna` deposit, records a weighted vote, and executes tallying: the first closes Market and the second reopens it. It verifies the `0.667` threshold, `PASSED` status, and parameter mutation only after adoption. Migration tests separately prove idempotent deposit-denom conversion.

On the persistent devnet, two regular proposals validated the complete network path for parameter changes:

| Proposal | Action | Result | Submission transaction |
|---|---|---|---|
| 1 | `0.0035` to `1` | `PASSED`, `YES` power: `900,000,000,000` | `344A0A20…90DB` |
| 2 | `1` to `0.0035` | `PASSED`, `YES` power: `900,000,000,000` | `FEAEC621…E614` |

The node was restarted after proposal 2 and retained `0.0035`. The expedited path was not replayed on this older volume because its state specifically retained the incorrect `stake` deposit; it is covered by the full integration test and the corrected v15 migration.

E2E-003 subsequently repeated the expedited path on a fresh four-validator network. Its 60%/40% vote did not pass the `0.667` expedited threshold and caused no parameter mutation. Separate 70%/30% votes then closed and reopened Market. The test also documents that expedited-to-regular conversion consumes the first votes and therefore requires validators to vote again during the regular period.

### 10. Remaining Improvements

Design gaps GAP-001 through GAP-004 are all addressed locally. No identified MM2 mechanism remains absent in this series. Open limitations concern publication, upgrade from a real snapshot, and long-running economic validation as described below.

### 11. Campaign Limitations

The base multi-validator harness, unequal-power Oracle outage scenario, and expedited governance brake are validated. The following points still need to be exercised by extending the harness or using a public environment:

1. network TWAP revalidation with irregular prices and a changing majority;
2. repetition of the corrected upgrade from a realistic pre-v15 snapshot, followed by activation at the next epoch;
3. truly parallel load from multiple accounts and multiple proposers;
4. a long economic campaign spanning several simulated production epochs.

These remaining tests do not change the current verdict. The P0 defects were reproducible or deterministic from the execution path and are now corrected locally.

### 12. Prioritized Recommendations

#### P0 — Before Any Community Test

1. Review and merge the feeder correction for `UST` (`USD per USTC`), then retain reciprocal integration coverage with the core.
2. Review and merge the `InitGenesis` and upgrade correction that guarantees a real `market_accumulator` `ModuleAccount`, including address collisions.
3. Review and merge the locally validated idempotent v15 migration, then repeat it against a representative pre-v15 snapshot.
4. Keep the blocking tests that require Market to remain disabled for a complete epoch and both reserves to be funded before first activation.

#### P1 — Before a Mainnet Proposal

1. Review the local `base_pool`/PRP implementation with its 7% adaptive / 10% strict-cap separation, then replay it across several epochs.
2. Review the local 25-block quorum auto-halt and retain the now-passing unequal-power network regression.
3. Review the local weighted TWAP, then confirm network blocking during bootstrap and recovery on block 45.
4. Review the expedited-deposit normalization to `uluna` and retain the now-passing 60%/70% multi-validator governance regression.

#### P2 — Operational Quality

1. Provide reproducible scripts for multi-account load, export/import, and evidence collection.
2. Clearly document mainnet, testnet, and devnet values so a 100-block epoch cannot be reused outside local testing.

### 13. Validation Resumption Criteria

A future campaign may conclude “GO for community testnet” when:

- each of the three P0 items has a regression test and its correction has been reviewed and published;
- a pre-v15 upgrade produces blocks, taxes, and swaps without panic while Market starts disabled;
- LUNC/USTC quotes match the two raw USD prices in both directions;
- the multi-validator network demonstrates halt, persistence, and recovery across quorum thresholds (**satisfied locally by E2E-002**);
- the multi-validator network demonstrates expedited closure at 100%, persistence, and reopening at 0.35% with `uluna` deposits (**satisfied locally by E2E-003**);
- the 7% adaptive / 10% strict-cap separation is documented, liquidity adaptation satisfies proposal bounds over several epochs, and the TWAP is truly duration-weighted.

### 14. Confidentiality and Reproducibility

This document contains no mnemonic phrase, private key, or Docker secret. Local-chain addresses and transactions are test data only. The local validator key used for the import smoke test was copied to `/private/tmp`, mounted read-only, and then deleted. Test exports remain outside the repository.

The Docker devnet is external to the MM2 repository. The initial campaign node used an image built from the frozen core revision; only the feeder image had been rebuilt at that stage. Core corrections are now clearly separated into reviewable commits on the published branch, and both draft PRs link the corresponding test evidence.

### 15. Final Conclusion

The No-Mint core demonstrates a promising foundation: transfers from a real pool, absence of Market minting, fee and epoch-end burns, 60% tax routing, caps, and atomic rejections all work in the local profile. Go suites and fuzzing revealed no general regression.

The code is nevertheless not yet ready for public community testing. `INT-001`, `INT-002`, `INT-003`, and `GAP-001` through `GAP-004` are corrected or reclassified and locally validated, and their draft reviews are open. They still require maintainer review, merge, and release. The Oracle/TWAP/Market path now operates across four unequal-power local validators through healthy operation, persistent quorum loss, recovery, expedited governance closure, and reopening. The proposal's 10% example should also be clarified.

**Recommended decision: community NO-GO in the current state.** The next reasonable steps are review and integration of the draft changes, an upgrade test against a representative snapshot, and longer economic validation. This report provides the baseline and criteria needed to measure that progress unambiguously.
