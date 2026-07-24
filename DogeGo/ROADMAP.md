# DogeGo - long-term roadmap (toward a Go full node)

This document tracks **incremental** work toward a Dogecoin-compatible node in Go. **Consensus parity with Dogecoin Core** remains the long-term bar; the checkboxes below include **MVP scaffolds** now present in the tree (experimental, not production-ready).

## Dogecoin protocol lock (no consensus forks)

DogeGo integrates Dogecoin Core behavior; it does **not** ship an alternate mainnet protocol.

| Constraint | Policy |
|------------|--------|
| **Mainnet consensus** | Block, header, script, subsidy, auxpow, and difficulty rules follow Dogecoin Core (`src/consensus`, `chainparams`). Any mainnet divergence is treated as a **bug**, not a feature. |
| **"Fork" in code/docs** | Means **chain reorganization** (competing tips), not a **protocol fork** or new activation rules. |
| **Reboot testnet** | Separate `network=testnet` with documented founder params; never substitutes for mainnet rules. |
| **PQ / experimental wallet RPC** | Optional OP_RETURN recognition and off-chain carrier metadata only; **no consensus rule change** without explicit Core-aligned review and tests. |
| **Witness / segwit** | Rejected at admission (effective policy aligned with Dogecoin mainnet today). |

### Security and protocol audits (ongoing)

| Layer | Gate |
|-------|------|
| **Offline consensus** | `dogego cert offline`, Milestone A differential harness, `script_tests.json` legacy corpus, mainnet field evidence (`dogego cert field-evidence`) |
| **Mempool / relay policy** | 58-template offline corpus + live reboottestnet probes; Core side-by-side when `DOGEGO_CORE_COMPARE=1` |
| **Operator / RPC** | `dogego cert operator`, `GET /api/core-operator-cert` (17 live web gates), `GET /api/core-compare` |
| **Live runner** | `dogego-live`: `weekly-live` Core 24/24 gate, wallet.dat import, optional Milestone B soak |
| **Threat model** | [docs/SECURITY.md](docs/SECURITY.md); non-consensus gaps in [docs/INTENTIONAL_DIFFERENCES.md](docs/INTENTIONAL_DIFFERENCES.md) |
| **Formal audit** | Phase 9 backlog: third-party review before high-value production claims |

**Optional self-certification (DogeGo native storage):** long-haul soaks (`dogego cert live-soak`, weekly-live, workflow 10) prove resilience of DogeGo’s Go layout (`headers/`, `rawblocks/`, `utxo.cache`, Pebble `wallet.db`) - not missing Core features. Tools remain under Features and `dogego cert`; they are not open Core-parity gaps.

Verifier-side PQ (OP_RETURN + carrier) + `dogego cert pq` + `GET /api/core-pq-probe` ship today; more soak testing welcome; not a consensus fork.

### How to read the checkboxes

| Label | Meaning |
|-------|---------|
| **[x] Done** | Shipped in DogeGo’s layout (`headers.bin`, `rawblocks/`, in-memory mempool, HD `wallet.json`, …). |
| **[x] Declined / intentional** | e.g. native USB/HID without HWI. Keypool behavior matches Core but stores in `wallet.json` (not `wallet.dat` BDB). |
| **Optional self-cert** | Live multi-hour soak / weekly-live / workflow 10 certify DogeGo’s native DB resilience; available via Features + `dogego cert`. |
| **Phases 1-3** | Sync path is complete for typical use. |

**Effort (rough):** S = hours-1 day · M = days · L = weeks · XL = separate project.

### Production goals (mainnet + reboot testnet)

Operator-facing bar for “works like a full Dogecoin node” on **both** networks:

| Goal | Status | Next work |
|------|--------|-----------|
| **Mainnet headers + bodies IBD** | Done; segment header storage + checkpoint; Core-style dedicated header IBD; archival peer preference for early blocks; auto journal recovery; forward body fill; local genesis from chainparams; dashboard tip from disk; **UTXO snapshot body replay** + contiguous ramp on restart (`node/replay_ramp_worker.go`, `ShouldPreserveContiguousCache`) | `dogego cert operational`; `docs/MAINNET_TESTNET_OPERATIONAL.md`; **`dogego cert ibd-convergence`** |
| **Reboot testnet** (`network=testnet`) | MVP; solo founder + real scrypt mining (`RelaxedPoW=false`); **modern params** - Digishield + Digishield min-diff from block **1**, **10k DOGE** tail subsidy from height **1** (genesis **88**); solo `mine=true` default on wizard | `dogego cert operational`; `dogego cert founder` + `GET /api/core-founder-probe`; dual-run wizard profile |
| **Mining** | Done (cert) | Mainnet: `generatetoaddress` (scrypt); aux era: `createauxblock` / `submitauxblock`; **`getblocktemplate`** Digishield `NextBlockBits` + BIP22 longpoll; testnet: `mine=true` background miner **includes mempool txs**; **Settings → Services** start/stop/restart solo miner; **`GET /api/core-mining-probe`** + `dogego cert mining` + `scripts/core_mining_workflow.ps1` (optional Core GBT compare). |
| **DGR / CGNAT relay** | MVP phases 1-4 | Bidirectional QUIC relay; persistent P2P tunnel pool; TLS pins; publish/push fan-in (`inv`/`tx`/`block`/`headers`); see [docs/DOGEGO_RELAY_CGNAT.md](docs/DOGEGO_RELAY_CGNAT.md) |
| **Tx index + explorer** | MVP when `-no_tx_index` off | `reindextx` after corruption; optional `dogego indexer` sidecar for heavy analytics |
| **Wallet mainnet + testnet** | Done (encrypt, unlock, history, coin control, address book; Core wallet.dat import) | Core-like keypool in HD `wallet.json` (not wallet.dat BDB); HWI via `signer_cmd` (native USB/HID declined) |
| **PQ commitments** | Done (OP_RETURN + TX_C/TX_R carrier; wallet flags; web Send; GET /api/core-pq-probe; dogego cert pq) | More soak testing welcome; not a consensus fork. |
| **OS login autostart** | Done (`autostart`: `login` \| `disable` in `dogecoinconf.json`) | `dogego cert autostart`; restart-resume probe + `core_restart_resume_check.ps1` when `autostart=login` |
| **Optional self-cert (native DB)** | Offline crash suite + Features probes done | Optional: `dogego cert live-soak` / weekly-live / workflow 10 for long-haul resilience of Go storage (not a missing Core feature) |

---

## Standalone full-node acceptance gate

To decide when DogeGo is truly standalone-ready, use:

- [docs/STANDALONE_FULLNODE_ACCEPTANCE.md](docs/STANDALONE_FULLNODE_ACCEPTANCE.md)
- [docs/CORE_PARITY_GAPS.md](docs/CORE_PARITY_GAPS.md) - operator-facing summary of unchecked Core parity (mirrors Features tab backlog)
- Cross-platform offline certification: **`dogego cert offline`** (`offlinegate/`; `scripts/ci_offline_gate.{ps1,sh}`), **`dogego cert operator`** (`operatorworkflow/`; `scripts/operator_workflow_cert.{ps1,sh}`), **`dogego cert wallet-import`** (`walletimport/`; `scripts/wallet_import_cert.{ps1,sh}`), **`dogego cert wallet-migration`** (`walletmigration/`; `scripts/wallet_migration_cert.{ps1,sh}`), **`dogego cert field-evidence`** (`fieldevidence/`; `scripts/field_evidence_cert.{ps1,sh}`), **`dogego cert pq`** (`pqcert/`; `scripts/pq_cert.{ps1,sh}`) - drift-guarded via `docs/scripts_cert_test.go`

This matrix is now the release gate for "Core-equivalent full node" status (consensus behavior, crash/recovery guarantees, P2P robustness, mempool parity, and operator RPC workflows).

### Immediate execution milestones (thinking harder track)

- [x] **Milestone A (consensus differential harness, scaffold):** fixture-driven diff runner for Core difficulty vectors (`consensus/core_differential_harness_test.go`, `consensus/testdata/core_difficulty_vectors.json`).
- [x] **Milestone A (header differential, partial):** fixture-driven checkpoint/stored/**segment_stored**/batch vectors through **six-hundred-header** monolith/segment (`journal_length`/`batch_length` compact fixtures), **forty-eight-header** batch accept, batch/journal bad-nBits index **24+**, batch bad-prev index **6+**, batch time MTP/new rejects, partial stored/segment ranges (`consensus/core_header_differential_harness_test.go`, `consensus/testdata/core_header_vectors.json`).
- [x] **Milestone A (block differential, partial):** fixture-driven `CheckBlockPayload` + stored genesis/…/**five-hundred-twelve**-block connect (`chain_tip_height` compact fixtures) + Core hex genesis/block-one + **mainnet hex genesis** import + **bad-cb-*/vin-empty/vout-*/prevout-null/header-hash/duplicate-spend/txouttotal/witness/unspendable/oversize-tx** reject (`source=hex` / `chain_genesis` in `core_block_vectors.json`)
- [x] **Milestone A (consensus differential harness, legacy script corpus):** full legacy `script_tests.json` execution (**1059/1059** rows pass; **witness rows intentionally declined** per segwit-disabled policy); multi-height stored chains through **512** blocks; **mainnet field evidence** - header PoW (`TestCoreMainnetFieldHeaderPoW`), valid-PoW block hex + connect (`mainnet_field_blocks.json`, `TestCoreMainnetFieldStoredBlockConnect`), export via `scripts/export_mainnet_field_blocks.ps1`.
- [x] **Milestone A (mainnet field evidence cert, partial):** offline connect corpus **0-3** + sparse coinbase **100/200/272/10006** + multi-tx **15504** CheckBlock (committed `testdata/mainnet_field_block_15504.hex`); bundled store round-trip/contiguous/measure/validate; **committed auxpow proofs** **371337-371339** + **legacy scrypt field_header 371340** (post-activation); aux export falls back to raw block CAuxPow when `headers_aux.bin` slot empty; `field_evidence_cert.ps1 -TryExportAuxpow`; **`dogego cert field-evidence`**; **`GET /api/core-field-evidence-probe`** + **`field_evidence_live_cert.ps1`** (graceful skip without datadir); disk connect (`field_disk_connect_cert.ps1`, incremental ancestor frontier, **ReconcileBundledContiguousTip** for locator/blk drift); drift offline gates (`TestReconcileBundledContiguousTipMeasuredAboveProbe`, `TestReconcileBundledContiguousTipMeasuredBelowProbe`); differential gates + `scripts/field_evidence_cert.ps1`
- [x] **Milestone A (script VM production fallback, partial):** `verifyInputEval` + `EvalScript` fallback for non-template `scriptPubKey` (`script_eval_verify.go`, `TestVerifyScriptEvalSimpleTrue`).
- [x] **Milestone A (script template corpus, partial):** P2PKH/P2PK/P2SH/multisig/CLTV fixture vectors + Core `script_tests.json` size catalog (`consensus/core_script_differential_harness_test.go`, `core_script_tests_catalog_test.go`).
- [x] **Milestone A (script stack interpreter, partial):** `ParseScriptASM` + `EvalScript`/`VerifyScriptTest`/`VerifyScriptTestSpend` (CHECKSIG/CHECKMULTISIG/HASH160/RIPEMD160/SHA256/HASH256/P2SH/CLTV/CSV/**IF/ELSE/NOTIF/ENDIF/VERIFY/**NOP/**VER/**RESERVED**/1NEGATE/**OP_16**/TOALTSTACK/FROMALTSTACK/PICK/ROLL/DEPTH/DROP/**2DROP**/DUP/OVER/2DUP/3DUP/2OVER/2SWAP/2ROT/IFDUP/SWAP/TUCK/NIP/EQUAL/EQUALVERIFY/NUMEQUALVERIFY/SIZE/NOT/NEGATE/**ABS/**0NOTEQUAL**/ADD/SUB/1SUB/1ADD/BOOLAND/BOOLOR/NUMEQUAL/NUMNOTEQUAL/LESSTHAN/GREATERTHAN/LESSTHANOREQUAL/GREATERTHANOREQUAL/MIN/MAX/WITHIN/CODESEPARATOR**/OP_RETURN**/ROT/disabled-opcodes/**OP_9/OP_11-OP_14**/stack underflow corpus) + differential stack/conditional vectors + `TestCoreSighashDifferentialHarness` (Core `sighash.json`) + `TestCoreScriptTestsRunnerSubset` (**1059/1059** legacy rows; witness declined) + `TestCoreScriptTestsWitnessRowsIntentionallyDeclined`.
- [x] **Milestone A (DER lax + BIP66, partial):** Core `ecdsa_signature_parse_der_lax` (`der_lax.go`); dedicated **DERSIG/BIP66/padding** corpus from `script_tests.json` (`TestCoreScriptTestsDERSIGCorpus`, `der_lax_test.go`, `sig_encoding_test.go`); pre-BIP66 lax verify + post-BIP66 `SCRIPT_VERIFY_DERSIG` strict path; `lenbyte>=8` multi-byte length guard (`TestDerLaxToCompactVectors`)
- [x] **Milestone A (reorg fork election, partial):** multi-branch adversarial fork storm - `maxAlternateForkWork` picks strongest peer alternate; `TestForkElectionMultiBranchStorm` (`node/header_chain_election_test.go`)
- [x] **Milestone A (reorg fork election, partial):** mock relay peer probe - `TestEnsureIncomingForkWins_rejectsPeerAlternate` / `_acceptsIncomingWinner` (`node/header_chain_election_mock_test.go`)
- [x] **Milestone C (UTXO hash + replay, partial):** `hash_serialized` stable across linear apply, `RebuildFromChain`, reorg-style replay, and `TruncateChainToHeight` UTXO rebuild (`store/utxo_hash_differential_test.go`, `node/chain_truncate_utxo_test.go`).
- [x] **Milestone B (crash/corruption suite, partial):** automated kill-and-restart/fault-injection tests for headers/raw/index/filter recovery, proving no-manual-step convergence (offline + live inject/soak scripts; extended runtime soak remains backlog).
- [x] **Milestone C (chainstate/reorg fidelity, partial):** deterministic UTXO `hash_serialized` across rewind/replay stress (`store/utxo_reorg_stress_test.go`, `node/chain_reorg_utxo_stress_test.go`).
- [x] **Milestone D (policy parity runner, partial):** fixture-driven mempool admission + reject-reason vectors (`consensus/core_mempool_differential_harness_test.go`, `consensus/testdata/core_mempool_vectors.json`).
- [x] **Milestone D (policy parity runner, partial):** all **58** `core_mempool_vectors.json` templates including P2SH nested/multisig, bare multisig, CLTV/CSV+P2PK script-checked spends, P2PK non-standard-input reject, dust/witness/bare-multisig-output/op_return/**unspendable-output/op_return-zero-accept/absurd-fee/multi-op-return/version/scriptsig-not-pushonly/non-final/tx-size-small/scriptsig-size/discourage-nop/op_return-oversize/p2sh-sigops/non-standard-output/datacarrier-disabled/p2sh-redeem-missing/discourage-nop1/discourage-nop6/rbf-too-many-descendants/non-bip68-final/tx-version-zero**/vin-empty/vout-empty/vout-negative/vout-toolarge/prevout-null/vout-empty-scriptpubkey/txouttotal-toolarge/tx-oversize** rejects, **FullRBF accept** (`rbf_fullrbf`), **BIP125 rule 2/5** (`rbf_too_many_conflicts`, `rbf_new_unconfirmed_input`); RPC `testmempoolaccept` covers corpus
- [x] **Milestone E (reindex workflow probe, partial):** `scripts/core_reindex_prune_workflow.ps1` - `getindexinfo`, maintenance RPC presence; optional testnet `reindextx` (`DOGEGO_REINDEX_PROBE=1`)
- [x] **Milestone D (policy parity runner, partial):** CSV timelocks + non-standard rejects (dust, witness, bare-multisig output, non-zero OP_RETURN) in harness + RPC differential path; full script matrix remains backlog
- [x] **Milestone D (live mempool RPC probe, partial):** **32** stateless `testmempoolaccept` rows in `mempool_parity_rpc.json` + `GET /api/mempool/parity-probe` (Core side-by-side when reachable); **full 58-vector offline probe** via `GET /api/mempool/parity-probe?corpus=full` + `consensus.EvalMempoolCorpus` (`TestEvalMempoolCorpus`)
- [x] **Milestone D (policy parity runner, partial):** full **58**-template offline corpus eval (`?corpus=full` / `?corpus=stateful`); live stateful on reboottestnet partial (see below)
- [x] **Milestone D (policy parity runner, partial):** live stateful probes on reboottestnet - **dust**, **absurd-fee**, **non-final**, **RBF-insufficient**, **RBF-sufficient**, **coinbase-immature**, **package-ancestor-limit** (`mempool_stateful_parity_reboottestnet.ps1 -Scenario all`)
- [x] **Milestone D (policy parity runner, partial):** offline **stateful** mempool corpus gate - `TestEvalMempoolCorpusStateful` (**24+** templates needing seeded pool/UTXO/package graph; complements **32** stateless live RPC rows)
- [x] **Milestone D (policy parity runner, partial):** live stateful reboottestnet probe - **22** scenarios (`mempool_stateful_parity_reboottestnet.ps1`: dust/RBF/package/min-relay/double-spend/coinbase + **P2SH nested/multisig/bare-multisig/CLTV/CSV**, **package size**, **p2pk-non-standard-input**; wallet-anchored via `cmd/statefulprobe` + `consensus/stateful_live_probe.go`; `mempool_stateful_live_map.go` + `TestStatefulMempoolLiveMapCoversKeyTemplates`)
- [x] **Milestone D (policy parity runner, partial):** full **24/24** stateful template live map (offline `TestEvalMempoolCorpusStateful` + live `-Scenario all`)
- [x] **Milestone D (policy parity runner, partial):** live stateful reboottestnet Core side-by-side when `DOGEGO_CORE_COMPARE=1` (`Compare-CoreMempoolRow` in `mempool_stateful_parity_reboottestnet.ps1`; fails cert on allowed/reject drift)
- [x] **Milestone D (policy parity runner, partial):** required Core compare gate - `DOGEGO_CORE_COMPARE_REQUIRED=1` + `scripts/mempool_stateful_core_gate.ps1` (fails when Core unreachable or drift)
- [x] **Milestone D (policy parity runner, partial):** `p2pk_non_standard_input` live via `submitblock` mined prep (`consensus/relaxed_block_mine.go`, `statefulprobe -submitblock`)
- [x] **Milestone D (policy parity runner, partial):** `core_reboottestnet_core_aligned_gate.ps1` + `DOGEGO_CORE_COMPARE_MIN=24`; GitHub Actions `live-core-gate` job (`DOGEGO_SCHEDULED_CORE_GATE=1` repo var)
- [x] **Milestone D (policy parity runner, partial):** `setup_reboottestnet_core_parity.ps1` + `ci_runner_preflight.ps1` - runner/Core wallet bootstrap before 24/24 gate; cross-platform **`dogego cert setup-parity`** (`runner/setup_parity.go`)
- [x] **Milestone D (policy parity runner, partial):** weekly live wallet.dat RPC **probe** in preflight and **import** after reboottestnet setup when `DOGEGO_WALLET_DAT` / `DOGEGO_WALLET_DAT_REQUIRED=1` (`dogego cert weekly`, `ci_runner_preflight.ps1 -live-probe`, `ci_scheduled_weekly_live.ps1`, `wallet_migration_cert.ps1 -SkipOffline`)
- [x] **Milestone D (policy parity runner, full):** offline 58-template + stateful 24/24 map done; **optional self-cert:** live Core side-by-side on `dogego-live` via **`dogego cert weekly-live`** (Features tab / workflow 10).
- [x] **Milestone B (IBD restart contiguous preserve, partial):** `ShouldPreserveContiguousCache` / `MaybeResetContiguousAfterHeaderRewind` skip collapse when UTXO or stored bodies ahead of checkpoint; multi-pass disk ramp + replay ramp worker (`node/header_rewind_contiguous_test.go`, `node/replay_ramp_worker.go`)
- [x] **Milestone B (UTXO snapshot quarantine, partial):** distinguish fabricated vs real replay snapshots; `TryRestoreBestQuarantinedUtxoSnapshot` + `scripts/restore_utxo_snapshot.ps1` (`node/utxo_snapshot_guard_startup_test.go`)
- [x] **Milestone B (crash/corruption suite, partial):** bundled `blk00000.dat` torn-tail clamp via `ProbeBundledContiguousTip` in startup + `autoRecoverSweep` (`maybeClampBundledContiguousFromDisk`); offline `TestAutoRecoverSweepClampsBundledContiguousAfterTornTail`
- [x] **Milestone B (crash/corruption suite, partial):** raw stub purge, header tail mid-write repair, index artifact tolerance (`node/crash_*.go`, `node/auto_recovery_test.go`).
- [x] **Milestone B (header segment crash recovery, partial):** segment tail repair on open, stale `.tmp` purge, checkpoint realignment, monolith auto-migrate (`store/header_segments.go`, `store/header_chain.go`, `node/crash_header_segments_test.go`).
- [x] **Milestone B (crash/corruption suite, partial):** active-write kill simulation - raw Put phases (.tmp complete/partial, undersized .bin), header segment/manifest `.tmp` discard, restart convergence (`store/crash_active_write_test.go`, `node/crash_active_put_restart_test.go`)
- [x] **Milestone B (headers_aux torn tail, partial):** truncate partial aux record on open; `autoRecoverSweep` reopens `headers_aux.bin` for repair (`store/header_aux.go`, `node/crash_header_aux_test.go`)
- [x] **Milestone B (crash/corruption suite, partial):** simulated kill before raw Put rename (`abortBeforeRawPutRename`, `TestCrashKillBeforeRawPutRename`)
- [x] **Milestone B (crash/corruption suite, partial):** OS subprocess kill mid-Put + mid header segment append (`TestSubprocessKillDuringRawPut`, `TestSubprocessKillDuringHeaderSegmentAppend`, `store/rawblock_subprocess_kill_test.go`)
- [x] **Milestone B (crash/corruption suite, partial):** `scripts/corruption_soak_cert.ps1` - subprocess kill + header segment tail/checkpoint/tmp recovery + bundled torn-tail reopen + mainnet field crash fixtures + startup recovery offline gate (`DOGEGO_CORRUPTION_SOAK=1` in `core_operator_workflow_cert.ps1`)
- [x] **Milestone B (timed IBD soak, partial):** `scripts/ibd_timed_soak.ps1` - repeated health/convergence window (`DOGEGO_TIMED_SOAK=1`)
- [x] **Milestone B (extended operator soak, partial):** `scripts/extended_operator_soak.ps1` - timed IBD + corruption inject soak with **`-CorruptionCycles`** + per-cycle `verifychain 2 0` (`DOGEGO_EXTENDED_SOAK=1`)
- [x] **Milestone B (crash/corruption suite, partial):** `scripts/corruption_inject_live.ps1` - live headers (seg + monolith)/raw/**bundled** (`blk00000.dat`)/filter/**txindex** tail truncate + restart RPC recovery (`DOGEGO_CORRUPTION_INJECT=1`, `-Target headers|raw|bundled|filter|txindex`; reboottestnet default)
- [x] **Milestone B (crash/corruption suite, partial):** `scripts/corruption_inject_soak.ps1` - multi-target live inject soak with **block-height convergence summary** (`DOGEGO_CORRUPTION_INJECT_SOAK=1` in `core_operator_workflow_cert.ps1`)
- [x] **Milestone B (crash/corruption suite, partial):** `scripts/corruption_timed_loop.ps1` - multi-round timed corruption inject with per-round `verifychain` + block-height guard (`DOGEGO_CORRUPTION_TIMED_LOOP=1`); **`corruption_timed_loop_mini.ps1`** for short cert (`DOGEGO_CORRUPTION_TIMED_MINI=1`); **`corruption_extended_cert_mini.ps1`** - health soak + timed loop on headers/raw/filter/txindex (`DOGEGO_CORRUPTION_EXTENDED_MINI=1`)
- [x] **Milestone B (crash/corruption suite, partial):** `scripts/corruption_long_soak_gate.ps1` - extended timed corruption on headers/raw/filter/txindex with health pre-soak (`DOGEGO_CORRUPTION_LONG_SOAK=1`; `DOGEGO_CORRUPTION_LONG_MIN` override)
- [x] **Milestone B (crash/corruption suite, partial):** GitHub Actions `.github/workflows/dogego.yml` - offline gates on push/PR (`ci_offline_gate.sh` / `.ps1`: docs/UI, mempool corpus, RPC recovery, **wallet UTXO-cache fast path**, **Core wallet.dat migration** synthetic fixtures, operator workflow, store corruption); optional scheduled live soak via `workflow_dispatch` / `DOGEGO_SCHEDULED_LIVE_SOAK` on `dogego-live` runner (`ci_scheduled_corruption_soak.ps1`)
- [x] **Milestone B (crash/corruption suite, partial):** `scripts/ci_live_reboottestnet_gate.ps1` - live health + E2E + Core-aligned + corruption mini (`DOGEGO_CI_LIVE_GATE=1`; GHA `live-reboottestnet` job)
- [x] **Milestone B (crash/corruption suite, partial):** `scripts/ci_runner_preflight.ps1` + `ci_runner_provision_checklist.ps1` - dogego-live runner readiness (GHA preflight step on live jobs; cross-platform TCP port probe)
- [x] **Milestone B/D/E (scheduled weekly live, partial):** cross-platform **`dogego cert weekly-live`** + **`dogego cert live-soak`** (`runner/weekly_live.go`, `runner/live_soak.go`; mirrors `ci_scheduled_weekly_live.ps1` / `ci_milestone_b_full_gate.ps1`)
- [x] **Milestone B/D/E (scheduled weekly live, partial):** `scripts/ci_scheduled_weekly_live.ps1` + GHA `live-weekly` job (`DOGEGO_SCHEDULED_WEEKLY_LIVE=1`; Core 24/24 + corruption mini); `gh_enable_scheduled_live.ps1` sets repo vars
- [x] **Milestone B (crash/corruption suite, full):** offline kill/repair + utxo.cache quarantine done; **optional self-cert:** multi-hour timed inject via **`dogego cert live-soak`** (proves DogeGo native storage resilience, not a Core feature gap).
- [x] **Milestone E (operator workflow certification, partial):** offline tests + **`dogego cert operator`** (`operatorworkflow/verify.go`; mirrors `scripts/operator_workflow_cert.ps1`; includes field-evidence + wallet-import slices) (`node/operator_workflow_cert_test.go`, `rpc/truncatetoheight_test.go`).
- [x] **Milestone E (Core side-by-side probe, partial):** `scripts/core_parity_probe.ps1` compares `getblockchaininfo` + `verifychain` vs `dogecoin-cli` (`DOGEGO_CORE_COMPARE=1` in `ibd_soak_cert.ps1`); `scripts/core_compare_with_core.ps1` - Core on :22555, DogeGo on :22557; `getchaintips`, filter lag, IBD assist pool, `getzmqnotifications`
- [x] **Milestone E (restart resume probe, partial):** `scripts/core_restart_resume_check.ps1` - checkpoint vs contiguous bodies, assist pool during IBD, connect lag + boost tuning; **autostart=login** OS registration gate (`DOGEGO_RESTART_RESUME=1` in `core_operator_workflow_cert.ps1`)
- [x] **Milestone E (maintenance workflow probe, partial):** `scripts/core_maintenance_workflow.ps1` - `verifychain`, `getindexinfo`, `getchaintxstats` (+ optional Core window compare; `DOGEGO_MAINTENANCE_PROBE=1`)
- [x] **Milestone E (end-to-end workflow, partial):** `scripts/core_end_to_end_workflow.ps1` - bundles health + restart-resume (connect lag + boost in JSON steps) + maintenance + Milestone D `offline_corpus` / `bip125_offline` / `mempool_parity` + optional **`ibd_convergence`** + Core compare/wallet; in-process `GET /api/core-end-to-end-probe` mirrors those steps (incl. **`ibd_convergence`** snapshot when disk-only)
- [x] **Milestone E (wallet basics probe, partial):** `scripts/core_wallet_workflow.ps1` + `GET /api/core-wallet-probe` - `getwalletinfo`, `getbalance`, `getnewaddress`, `validateaddress`, `dogego_listwalletaddresses`, `setlabel`/`getaddressesbylabel`/`listlabels` round-trip when wallet enabled
- [x] **Milestone E (Core side-by-side full probe, partial):** `scripts/core_side_by_side_full.ps1` - parity + mempool + maintenance + restart-resume + end-to-end (`DOGEGO_CORE_COMPARE=1`)
- [x] **Milestone E (restart workflow probe, partial):** `scripts/core_restart_workflow.ps1` - stop/start via `restart_node.ps1` + resume invariants (`DOGEGO_RESTART_WORKFLOW=1`, disruptive)
- [x] **Milestone E (Core mempool RPC probe, partial):** stateless `testmempoolaccept` side-by-side vs Core - **32** rows (`scripts/core_mempool_parity_probe.ps1`, `consensus/testdata/mempool_parity_rpc.json`, `GET /api/mempool/parity-probe`)
- [x] **Milestone E (operator workflow certification, partial):** `scripts/core_mainnet_maintenance_compare.ps1` - read-only verifychain/getindexinfo/getchaintxstats side-by-side vs Core on mainnet
- [x] **Milestone E (operator workflow certification, partial):** `scripts/core_mainnet_side_by_side_runbook.ps1` - mainnet non-disruptive bundle (offline field evidence + 58-template mempool corpus + Core :22555 vs DogeGo :22557 compare, maintenance, wallet)
- [x] **Milestone E (operator workflow certification, partial):** `scripts/core_mainnet_restart_compare.ps1` - DogeGo mainnet restart with Core read-only alignment (`-AllowMainnet`)
- [x] **Milestone E (operator workflow certification, partial):** `scripts/core_mainnet_reindex_compare.ps1` + `scripts/core_reboottestnet_reindex_workflow.ps1` (`DOGEGO_REBOOTTESTNET_REINDEX=1`)
- [x] **Milestone E (operator workflow certification, partial):** `scripts/core_reindex_prune_disruptive_workflow.ps1` - disruptive `reindextx` / optional `reindexblockfilters` / reboottestnet `pruneblockchain` + verifychain + Core index compare (`DOGEGO_REINDEX_DISRUPTIVE=1`; mainnet requires `-AllowMainnet -ConfirmDisruptive`)
- [x] **Milestone E (operator workflow certification, partial):** `scripts/core_reboottestnet_reindex_compare.ps1` - read-only reboottestnet getindexinfo vs Core
- [x] **Milestone E (operator workflow certification, partial):** `scripts/core_mainnet_disruptive_reindex_gate.ps1` - mainnet operator sign-off (`-AllowMainnet -ConfirmDisruptive`; reindextx on DogeGo only, Core read-only compare)
- [x] **Milestone E (operator workflow certification, full):** scripts + 17 live web gates done; **optional self-cert:** disruptive mainnet reindex/prune on a production datadir (operator sign-off).
- [x] **Milestone E (operator workflow certification, partial):** `scripts/core_operator_runbook_full.ps1` - bundles all DOGEGO_* cert flags (offline `-OfflineOnly`; mainnet `-CoreCompare`)
- [x] **Milestone E (web UI live Core probes, partial):** loopback `/api/core-*` probes + Features/Overview operator cert matrix (**17** live web gates incl. autostart, founder, **runner readiness**, Milestone D **setup-parity**, **BIP152 HB**, **mining GBT/aux**, **PQ format**, **IBD convergence snapshot**, **addrman snapshot**, end-to-end); **90s probe cache** + startup warm; `?matrix=1` / `?refresh=1`; `GET /api/core-status` (Overview 60s poll; cached mempool corpus/parity); `GET /api/core-end-to-end-probe` (incl. `bip125_offline`, `mining`); Settings **Test Core connection** + cached cert strip; Console probe shortcuts; sync-dock operator cert + **UTXO body replay** during IBD
- [x] **Milestone E (reboottestnet E2E runbook, partial):** `scripts/core_e2e_reboottestnet_runbook.ps1` - health + restart-resume + maintenance + offline mempool corpus + wallet + stateful mempool `-Scenario all` (+ optional `-IncludeCoreCompare`, `-IncludeReindex`, `-IncludeRestartWorkflow`)
- [x] **Milestone E (reboottestnet full E2E runbook, partial):** `scripts/core_e2e_full_runbook.ps1` - offline operator cert + IBD convergence + reindex/prune check + recovery probe + wallet + stateful mempool (+ optional `-IncludeDisruptive`, `-IncludeCoreCompare`, `-IncludeCorruptionMini`)
- [x] **Milestone E (recovery workflow probe, partial):** `scripts/core_recovery_workflow.ps1` - `dogego_recoverheaders` RPC presence + header sync recovery fields (`DOGEGO_RECOVERY_PROBE=1`; `-InvokeRecover` disruptive)
- [x] **Milestone E (mainnet read-only E2E runbook, partial):** `scripts/core_e2e_mainnet_runbook.ps1` - offline corpus + mainnet side-by-side + maintenance + reindex compare (`-AllowMainnet`)
- [x] **Milestone E (operator workflow certification):** offline + web probes done; **optional self-cert:** scripted end-to-end on **dogego-live** via **`dogego cert workflow10`** / weekly-live (see `docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md` workflow 10).

### Certification exit checklist (`dogego-live` runner)

These milestones close when the self-hosted **`dogego-live`** runner runs green scheduled CI. Offline scaffolding (cert CLI, probes, drift guards) is complete; execution is operator-owned.

**Offline prerequisites** (any machine; no `dogego-live` required):

```powershell
go run ./cmd/dogego cert offline
go run ./cmd/dogego cert wallet-import
# optional PQ format/carrier slice (~40s; no production PQ safety claim):
go run ./cmd/dogego cert pq
# optional deep Milestone E sign-off (~5-20 min):
go run ./cmd/dogego cert operator
```

Bundle (same gates): **`scripts/cert_offline_prerequisites.{ps1,sh}`** (`-IncludePQ` / `-IncludeOperator` on PS1; `INCLUDE_PQ=1` / `INCLUDE_OPERATOR=1` on shell).

Use **`dogego cert wallet-migration`** for wallet.dat-only live probe/import (`-live-probe` / `-live-import` when `DOGEGO_WALLET_DAT` is set on dogego-live). Offline-only slice: **`dogego cert wallet-migration -offline-only`** (also used by `wallet_migration_cert.ps1`).

| Prerequisite | Command | Pass bar |
|--------------|---------|----------|
| **Offline CI gate** | `dogego cert offline` | All `offlinegate/` suites exit 0 (incl. wallet migration RPC `MixedPool` / `NativePoolIndicesReplayed`) |
| **Wallet.dat fixtures** | `dogego cert wallet-migration` | BDB probe/extract/import E2E; optional `-live-probe` / `-live-import` when node + `DOGEGO_WALLET_DAT` set |
| **Wallet import (extended)** | `dogego cert wallet-import` | BIP39/BIP38 + signer + UI/RPC import paths; includes wallet-migration offline suites |
| **Milestone E operator (deep)** | `dogego cert operator` | Core consensus/store/node/rpc + field-evidence + wallet-import (~5-20 min; `-skip-field-evidence` / `-skip-wallet-import` for slices) |
| **PQ format/carrier (optional)** | `dogego cert pq` | OP_RETURN commitment + TX_C/TX_R carrier format tests; **no production PQ safety claim**; `scripts/pq_cert.ps1` / `scripts/pq_cert.sh` |

| Milestone | ROADMAP line | Command (workflow 10) | Pass bar |
|-----------|--------------|------------------------|----------|
| **E - workflow 10 sign-off** | §120 above | `dogego cert workflow10 -enable-github -github-apply -mine-bootstrap -require-wallet-dat` (or stepwise enable-weekly → provision → weekly-live → optional live-soak) | All stages exit 0; repo vars `DOGEGO_SCHEDULED_WEEKLY_LIVE=1` (+ optional `DOGEGO_SCHEDULED_LIVE_SOAK=1`) |
| **D - stateful Core 24/24** | §74 | `dogego cert weekly-live -mine-bootstrap -require-wallet-dat` (no `-skip-scripts`) | `core_reboottestnet_core_aligned_gate.ps1` reports **24/24**; wallet.dat import when required |
| **B - multi-hour soak** | §96 | `dogego cert live-soak -duration-min 60 -require-soak-env` | Timed corruption inject converges; `verifychain 4 0` after soak |
| **E - mainnet disruptive reindex** | §113 | `scripts/core_mainnet_disruptive_reindex_gate.ps1 -AllowMainnet -ConfirmDisruptive` | Manual operator sign-off on production datadir (script ready) |

Smoke on a dev machine (no PS1 Core gate): `dogego cert weekly-live -skip-scripts -mine-bootstrap -require-wallet-dat`.

---

## Phase 0 - Foundation

- [x] Chain constants / rebooted testnet alignment (`chain/`)
- [x] Dogecoin scrypt PoW (`pow/`, matches `src/crypto/scrypt.cpp` vectors)
- [x] P2P message framing + `version` payload (`wire/`)
- [x] CLI: `genesis`, `ping`, `node` (`cmd/dogego`); analytics side-DB via **`dogego indexer`** (`indexer/`)

---

## Phase 1 - P2P core

- [x] **MVP:** Single outbound peer, magic-aware `ReadMessage`/`WriteMessage`, `ping`/`pong` while syncing and in steady state (`node/`, `wire/`)
- [x] **MVP:** Multi-peer relay manager with **CGNAT-friendly modes** (`node/peermgr.go`, `node/p2p_mode.go`): `p2p_connectivity` / `-p2p` = `classic` (inbound listen + outbound), `cgnat` (outbound-only, no public listen - Starlink/carrier NAT), or `both` (default). Primary sync peer stays on the main loop; extra peers relay `tx`/`block`/`addr`/`feefilter`. Limits: `maxoutbound` (default **12**, includes primary), `maxinbound` (default **16** when listening).
- [x] **MVP:** learned peer addresses persisted to `learned_addrs.json` (load on start, periodic + shutdown save; cap 1000; `node/addrstore.go`)
- [x] **MVP:** `setban` enforced on **inbound** P2P accept when multi-peer listen is active (`MemoryBanManager.IsBanned`, `PeerMgr.SetBanChecker`)
- [x] **MVP:** block-download **peer scoring** (rank dial candidates, cooldown slow/failing peers; header pick + block-assist + **relay outbound** during IBD; persisted in `block_peer_scores.json`; **OrderCandidates read-only** for unseen addrs; exposed in **getpeerinfo** / web `/api/p2p` as `dogego_block_score`, `top_block_peers`)
- [x] **MVP:** outbound **ping/pong RTT** → `getpeerinfo` **`pingtime`** / **`minping`** / **`pingwait`** (primary + relay peers; ~2 min interval); **`relaytxes`** from version BIP37 byte; **`addnode`** / **`whitelisted`** (persistent addnode) / **`banscore`** (misbehavior, always present when tracker wired); **`inflight`** block heights per sync lane; **`feefilter`** as decimal DOGE string; **`bytesrecv_per_msg`** / **`bytessent_per_msg`** per P2P command; **`addr_processed`** / **`addr_rate_limited`** on inbound `addr`; **`lastsend`** / **`lastrecv`** from actual P2P activity (not RPC poll time); block-assist **`bytesrecv`**/**`bytessent`** per worker; header probe logs expected P2P magic on mismatch; per-peer **`feefilter`** (DOGE/kB) when peer sent BIP133 filter; **`timeoffset`** from peer version **nTime**; **`addrlocal`** from connection local endpoint; **`last_block`** / **`last_transaction`** unix times when peer delivers blocks or accepted txs (P2P getdata/broadcast + inv tx); header-sync **bad magic** tries next peer
- [x] **MVP:** **BIP37 bloom** (DIP-0037) - Core-compatible `bloom/` package; full-node `NODE_BLOOM` + `filterload`/`filteradd`/`filterclear` + `MSG_FILTERED_BLOCK`→`merkleblock` + bloom-gated tx relay; SPV wallet client `filterload` + filtered-block sync + merkleblock ingest (`node/spv_bloom.go`)
- [x] **MVP:** **addrbook v2** - learned_addrs with try/success/cooldown metadata; dial/feeler pick by score + block peer scorer (`node/addrbook.go`, `learned_addrs.json` version 2)
- [x] **MVP:** **misbehavior scoring** - invalid inbound headers / P2P reject / invalid blocks accumulate score; auto-`setban` at 100; BIP61 `reject` reply for bad blocks (`MisbehaviorTracker`, `EncodeReject`)
- [x] **MVP:** **persistent banlist** - `banlist.json` under chain datadir (survives restart; `setban` / misbehavior auto-ban / `clearbanned`) (`rpc/ban_persist.go`)
- [x] **MVP:** **`listbanned` prunes expired** entries (and rewrites `banlist.json` when using `FileBanManager`)
- [x] **MVP:** **misbehavior scores persisted** - `misbehavior_scores.json` under chain datadir (load at start, save on score + shutdown)
- [x] **MVP:** addrman **tried vs new** split - `PickBest` prefers tried after successful handshake; feelers probe new-only with group spread; **Core-scale flat caps** (256×64 tried / 1024×64 new); **hash-bucket spread** for tried-cap eviction (`addrbook_buckets.go`); **`dogego_addrbook_*_buckets_*`** on `getnetworkinfo`; 30-day stale prune; **`IsRoutable`** filter on learn/load/gossip/dial/`getaddr`; **CAddress nTime** clamp (no >10m future; ancient >30d → now); **/16 group** diversity on new-table dials (`dialScoreLocked`); `getaddr` via **`AddrSample`**; `learned_addrs.json` v2; **`dogego_addrbook_*`** on `getnetworkinfo` and `getblockchaininfo`
- [x] **Core parity:** addrman buckets (`src/net.*`) - **partial** (256/1024 hash buckets + **64-deep slot indices** + **multi-ref new ≤8** + **Core-scale table capacity**; **Core-style nKey** + **`learned_addrs.json` v3**; inbound eviction-when-full; **`GET /api/core-addrman-probe`** + churn soak in `node/addrbook_buckets_test.go`)
- [x] **MVP:** relay **peer eviction** (idle outbound relays) + **feeler** reachability probes; **header top-up** on primary after body IBD
- [x] **MVP:** **faster relay maintainer during IBD** (12s tick + extra feeler while bodies lag; relay peers fetch blocks on their lanes)
- [x] **MVP:** **primary auto-redial** on recoverable read errors during steady sync (ranked candidates / fixed `-peer` first then learned peers; `ReplacePrimary` + post-redial `feefilter` + header top-up; min-interval + streak cap; `node/primary_reconnect.go`)
- [x] **MVP:** configured **`peer` unreachable** or handshake failure → DNS/fixed-seed fallback (not fatal exit); **primary redial exhaustion** → background header recovery + block-assist (node stays up; `pausePrimaryForRecovery`, `StartHeaderSyncBackgroundRecoveryOnce`)
- [x] **MVP:** **defer header pull** when bodies lag and peer height ≫ local tip (`DownloadHeaders` returns success, block-assist continues); **startup with local headers** survives empty DNS / failed probe (`HasLocalHeaderChain`, background recovery)
- [x] **MVP:** **block-assist launch** after empty peer pool (`EnsureBlockAssistWorkers` mutex latch; assist candidates seeded on `header_catch_up_pending`); configured-peer **journal rewind** falls back to DNS seeds
- [x] **MVP:** **`dogego_recoverheaders`** / web recover restarts background header sync + block-assist (`headerRecoverKickCh`); operator rewind uses last sync error for auxpow/checkpoint-targeted recovery (`noteHeaderSyncFailure`); `header_catch_up_pending` is atomic (RPC-safe)
- [x] **MVP:** **checkpoint hash mismatch** at Core mapCheckpoints heights auto-rewinds journal (`maybeRewindOnCheckpointMismatch`); probe failure with local headers no longer fatal-exits (`headerCatchUpPending` skips inline sync)
- [x] **MVP:** **`dogego_block_assist_active`** on `getblockchaininfo` + web `/api/summary`; background header recovery wires **`RefreshDiscovery`** (DNS/fixed seeds)
- [x] **MVP:** **web + RPC `dogego_recoverheaders`** share **`afterHeaderJournalRewind`** (background header sync + block-assist kick); relay **header top-up** while primary unset during catch-up
- [x] **MVP:** **inbound `headers` policy** - local journal rewind does not misbehavior peer; auxpow/checkpoint/transport errors pause primary or top-up (`InboundHeadersErrorPolicy`); **`dogego_recoverheaders`** can restart sync when journal unchanged (`RestartHeaderSyncIfStuck`)
- [x] **MVP:** web **Recover header journal** uses in-process **`dogego_recoverheaders`** (same restart/kick path as Console RPC)
- [x] **MVP:** **background header sync** retries after local journal rewind (`shouldTryNextHeaderSyncPeer`); **marginal fork** headers ignored without misbehavior; **getheaders** rewind resets contiguous body cursor
- [x] **MVP:** **primary pause / background attach** refresh DNS peers + assist pool; post-attach header top-up (primary + relays) and `getaddr` when bodies lag
- [x] **MVP:** background header recovery **re-runs DNS/fixed-seed discovery** when peer list is empty or every few passes (`HeaderSyncRecoveryEnv.RefreshDiscovery`; main loop refresh while `header_catch_up_pending` without primary)
- [x] **MVP:** **header sync stays up** on transient P2P failures - `shouldAutoRecoverHeaderSync` + background recovery for transport/`bad prev`/auxpow peer errors (node + web UI keep running)
- [x] **MVP:** **background header catch-up** still runs **block-assist IBD** (assist workers + stall recovery + pool refresh; main loop no longer skips body sync when primary is unset)
- [x] **MVP:** **`dogego_header_catch_up_pending`** + **`headers_catching_up`** sync health when headers retry but block-assist is active (`rpc/sync_display.go`, web `/api/summary`)
- [x] **MVP:** block-assist workers read **dynamic primary exclude** (`PrimaryExclude`) so assist dials track primary redial
- [x] **MVP:** **block-assist peer pool refresh** every **90s** during IBD (`learned_addrs` + scorer history; `assist_peer_pool` in dashboard IBD stats)
- [x] **MVP:** **peer discovery feed** from inbound `addr` + periodic **`getaddr`** during IBD (feeds assist pool refresh and primary redial)
- [x] **MVP:** learn **`addr` during header sync**; **IBD stall recovery** (no block stored 15+ min → getaddr + assist pool refresh)
- [x] **MVP:** **reboot testnet modern consensus** - Digishield retarget from height **1**; **strict min-difficulty** from block **1** (not legacy 157 500 gate); **10 000 DOGE** block subsidy from height **1** (`fTailSubsidyOnly`; genesis coinbase **88**); aligned with Core `CTestNetParams` reboot flags (`consensus/dogeconsensus.go`, `consensus/subsidy_params.go`, `src/chainparams.cpp`)
- [x] **MVP:** `setmaxconnections` adjusts outbound cap when multi-peer mode is active (`node/peermgr.go`, RPC); **8-32** range; persists **`maxoutbound`** to `dogecoinconf.json`; Core **`RPC_CLIENT_P2P_DISABLED` (-31)** when P2P hooks absent
- [x] **MVP:** `addnode` / `disconnectnode` / `getaddednodeinfo` when multi-peer P2P is active (`node/addnode.go`, `node/peermgr.go`); **`addnode` add/remove persisted** to `dogecoinconf.json` `addnode` array; **addnode tried first** during header-sync peer probe and **outbound relay dials** (`AddrBook.PickBest` + `PeerMgr.SetPreferredPeers`)
- [x] **Core parity:** **OS firewall auto-setup** - `firewall` in `dogecoinconf.json` / `-firewall` (`auto`|`always`|`never`); Windows UAC / Linux ufw+firewalld / macOS socketfilterfw (`netfw/`, `node/firewall.go`)
- [x] **Core parity:** **UPnP / NAT-PMP port mapping** - `upnp` / `-upnp` (`auto`|`enable`|`disable`); IGD2 → IGD1 → NAT-PMP; `getnetworkinfo.localaddresses` public endpoint; 20m refresh (`netfw/upnp/`, `node/portmap.go`)
- [x] **Core parity:** **ZMQ PUB notifications** - `zmqpubhashblock` / `zmqpubhashtx` / `zmqpubrawblock` / `zmqpubrawtx` in `dogecoinconf.json`; pure-Go `zmq4`; multipart + LE sequence (`zmqnotify/`); **`getzmqnotifications`** RPC
- [x] **MVP:** optional **`dnsseed`** hostnames in `dogecoinconf.json` merged into chain params at startup (reboot testnet founder DNS; Core `CTestNetParams` has none)
- [x] **Core parity:** **`-dnsseed`** / **`dnsseed_lookup`** - disable DNS seed queries (fixed `chainparamsseeds.h` only); clearer log when Core DNS names do not resolve
- [x] **MVP:** **`maxorphantx`** in `dogecoinconf.json` (0 = default 100, cap 1000; Core `-maxorphantx` analogue)
- [x] **MVP:** **`getnettotals`** aggregates bytes across connected P2P sessions **and block-assist IBD workers**; Core **`uploadtarget`** stub (no bandwidth cap); **-31** when P2P counters unwired; CLI **`-maxorphantx`**
- [x] **MVP:** **`disconnectnode`** Core **-29** when peer not connected; host/port match via **`addnodeMatchesSession`**; primary sync peer cannot be disconnected
- [x] **MVP:** **`setban` / `listbanned` / `clearbanned` / `getpeerinfo` / `getconnectioncount`** use Core client RPC codes (**-23** duplicate ban, **-24** unknown addnode filter, **-29** disconnect, **-30** invalid/unban IP, **-31** P2P disabled)
- [x] **MVP:** **`getaddednodeinfo`** reports **`connected: true`** and live **`addresses[]`** with Core **`connected`: `"outbound"` / `"inbound"`** when addnode peer is primary, relay, or block-assist session
- [x] **MVP:** inbound **`addr`** gossip **token-bucket rate limit** (0.1 addr/s refill, 1000 cap; shuffle before process; **addnode whitelisted**; extra tokens after outbound **`getaddr`**; **`addr_processed`** / **`addr_rate_limited`** match Core semantics)
- [x] **Core parity:** **inbound handshake → addrbook** - routable inbound peers learned into **new** table + discovery feed (`Services` from version; not tried) (`peermgr.go` `noteInboundPeerLearned`)
- [x] **MVP:** **`sendcmpct`** (Dogecoin Core wire: `announce` bool + `version` uint64): negotiate BIP152 v1 HB (up to 3 peers); track **`bip152_hb_to`** / **`bip152_hb_from`**
- [x] **MVP:** inbound **`getdata`** → serve **`block`** / **`tx`** / **`cmpctblock`** (`MSG_CMPCT_BLOCK`) from `rawblocks/` + mempool (+ confirmed tx via `indexes/tx`); **`notfound`** for missing vectors (`node/getdata_serve.go`, primary + relay peers)
- [x] **MVP:** inbound **`getheaders`** → **`headers`** from local `headers.bin` (+ `headers_aux.bin` when auxpow version bit set; locator fork, max 2000, hashStop) (`node/getheaders_serve.go`, primary + relay peers)
- [x] **MVP:** BIP152 v1 compact-block relay (`cmpctblock`, `getblocktxn`, `blocktxn`, `MSG_CMPCT_BLOCK`); auxpow blocks use full `inv`/`block` (announce + getdata fallback); live probe `GET /api/core-bip152-probe` + `scripts/core_bip152_probe.ps1`; offline AuxPoW/cmpct edges + **`dogego cert bip152-soak`** (live timed soak still operator-owned via `bip152_live_soak_gate.ps1`)
- [x] **MVP:** short-lived header-sync / block-assist links still decline cmpct (`ReplySendCmpctDecline` on ephemeral peers)
- [x] **MVP:** ranked **`getaddr`** samples via block peer scores
- [x] **MVP:** web dashboard **IBD live stats** (blocks behind, blk/min, **est. time left**, mempool txs, in-flight batches, workers, **sync status line**) + Overview hero (Core-style phases: headers / blocks / synced) + periodic IBD log during catch-up (`/api/summary`, `rpc/sync_display.go`)
- [x] **MVP:** **DogeGo relay CGNAT (DGR phase 1)** - integrated QUIC UDP relay (`node/dgr/`); P2P service bit **`NODE_DOGEGO_RELAY_CGNAT`** (`1<<29`) on `version`/`addr`; config **`dogego_relay_cgnat`** (inbound operator / outbound CGNAT client); static seeds + multi-hostname **DNS TXT** + P2P peer discovery; DGR1 frames REGISTER/PING/**INV_TX**; metrics on **`GET /api/dgr`** + `/api/p2p`; [docs/DOGEGO_RELAY_CGNAT.md](docs/DOGEGO_RELAY_CGNAT.md)
- [x] **MVP:** **DGR phase 2** - **`P2P_FRAME`** TCP proxy with **persistent tunnel pool**; **`P2P_TUNNEL`** unsolicited peer push; client **`RelayP2PFrame`** request/response; **`PEER_HINT`** → persistent **`addnode`** on CGNAT clients; operator hints on **`REGISTER_OK`**; **`PeerMgr.SetDGRTunnel`** falls back to **DGR tunnel `net.Conn`** when outbound TCP dial fails (`node/dgr/p2p_frame.go`, `node/dgr/p2p_proxy.go`, `node/dgr/p2p_tunnel_pool.go`, `node/dgr_tunnel_push.go`, `node/dgr_tunnel_conn.go`, `peermgr.go`; **`getpeerinfo`** **`dogego_dgr_tunnel`**) - **L**
- [x] **MVP:** **DGR phase 2 polish** - **DGR boots before primary connect**; **`DialP2POutbound`** shared by primary, header probe, redial, and **PeerMgr**; **DGR tunnel-first** outbound P2P dial when **`outbound_relay`** is enabled (not only `p2p_connectivity=cgnat`); **relay seed addnode** on chain P2P port; primary **`dogego_dgr_tunnel`** on solo path (`node/dgr_dial.go`, `dgr_wiring.go`, `run.go`) - **M**
- [x] **MVP:** **DGR phase 3** - operator **`relay_tls_pins`** (SHA-256 cert fingerprint); inbound session **frame/P2P proxy rate limits** + **REGISTER per-IP cap**; relay **reputation in addrman** (DNS host:port targets; sort discovery by dial score; metrics **`tls_pin_*`**, **`rate_limited`**, **`p2p_publish_*`**, **`p2p_push_*`**, **`p2p_tunnel_*`**, **`server_cert_sha256`**) (`node/dgr/tls_pin.go`, `ratelimit.go`, `reputation.go`, `dgr_relay_book.go`) - **M**
- [x] **MVP:** **DGR phase 4** - **`P2P_PUBLISH`** client→operator→network (`inv`/`tx`/`block`); **`P2P_PUSH`** operator→client fan-in on primary + relay peers (`inv`/`tx`/`block`/`headers`); wallet tx + mined block relay via DGR; hostname resolve for tunnel dials (`node/dgr/publish.go`, `node/dgr_bridge.go`, `node/relay_session.go`, `run.go`) - **L**
- [x] **Operator UX:** **DGR phase 3 settings polish** - Settings **`relay_tls_pins`** + operator rate limits; live **`server_cert_sha256`** / TLS pin metrics on Overview + Settings; **`useServerCertPin`** helper (`st-dgr-tls-pins`, `index.html`, `app.js`) - **S**
- [x] **Operator UX:** **`dogego cert ibd-convergence`** - cross-platform forward IBD progress check (RPC + web + disk; mirrors **`scripts/ibd_convergence_check.ps1`**) (`ibdconvergence/`, `cmd/dogego/cert_ibd_convergence.go`) - **S**
- [x] **Operator UX:** **Features addrman snapshot card** - `GET /api/core-addrman-probe` + probe strip mini-pill + **16th** live cert gate + `scripts/core_addrman_workflow.ps1` (`ui/core_addrman_probe.go`, `ui/static/app.js`) - **S**
- [x] **Operator UX:** **Features IBD convergence snapshot card** - `GET /api/core-ibd-convergence-probe` + probe strip mini-pill + **15th** live cert gate (`ui/core_ibd_convergence_probe.go`, `ui/static/app.js`) - **S**
- [x] **Operator UX:** **`dogego cert operational`** + **`GET /api/core-operational-probe`** - mainnet / reboot testnet / dual-run config preflight (`operational/`, `docs/MAINNET_TESTNET_OPERATIONAL.md`) - **S**

---

## Phase 2 - Serialization (blocks & transactions)

- [x] **MVP:** Non-auxpow 80-byte header view (`primitives/`), `getheaders` / `headers` payloads (`wire/headers.go`)
- [x] **MVP:** `EncodeGetData`, on-disk raw block files (`store/rawblock.go`), genesis `getdata` → `block` after headers (`node/fetch.go` - `SyncGenesisRawBlock`; no full block parse yet)
- [x] **MVP:** raw block fetch for **genesis** plus a **tip window** of N headers (`SyncRecentRawBlocks`; default N=5, `rawblock_backfill` / `-rawblock_backfill`)
- [x] **MVP:** progressive `getdata` catch-up toward genesis (multi **block-assist** TCP workers + primary **lane 0**; height **stripes** + **rebalance** when a lane finishes early; `block_sync_workers` / derived from `maxoutbound` up to **24** lanes; batches of **32**; **MSG_BLOCK** retry after witness inv)
- [x] **MVP:** **forward IBD stripe cap** - when bodies lag headers by >512, parallel lanes only stripe within **4096 heights** above `lowMissing` (fixes assist workers jumping to ~tip/3 while height 2 is empty)
- [x] **MVP:** **forward IBD frontier-first** - all lanes share the same low-height window until contiguous bodies catch up; stale `rawblocks_sync.json` cannot skip the first gap (`claimBatch` + UI orphan raw estimate)
- [x] **MVP:** when **headers.bin** already has a chain, **arm block sync at startup** (`PrepareAtStartup`), fetch **genesis** before header catch-up if missing, run **block-assist** in parallel with `getheaders` (bounded header rounds when peer height ≤ local tip but bodies lag)
- [x] **MVP:** **interleaved block fetch on primary during header sync** when bodies lag (**5s** read timeout → up to **3** `getdata` batches per wait; `inv` block vectors → getdata)
- [x] **MVP:** defer startup **tip backfill** when contiguous raw bodies lag headers by >512 heights (Core-style forward IBD first); **auto tip backfill** when catch-up is close
- [x] **MVP:** **defer inbound `headers`** on primary + relay while forward block IBD is active (`ShouldDeferInboundHeaders`); **`inv` block announce** after contiguous connect when near header tip (`AnnounceBlockHash`)
- [x] **MVP:** after block download catches up, send BIP35 **`mempool`** to **all connected peers** for tx `inv` relay; **`feefilter`** on relay connect + periodic broadcast; **do not `inv` the peer that sent the tx** (`BroadcastTx` exclude source); relay **`block`** fan-out excludes source peer (`BroadcastCmd` exclude)
- [x] **MVP:** extend **UTXO cache** every **512** contiguous heights during bulk IBD (every **64** when caught up; + periodic snapshot)
- [x] **Core parity:** **forward IBD connect boost** - body-IBD lag≥512 → 4× connect passes + up to 128 blocks/call; lag≥8192 → 8× passes + `syncUtxoMaxConnectPasses` 512; **`dogego_connect_catch_up_{passes,batch,interval_ms}`** on `getblockchaininfo` / `/api/summary`; Overview IBD card + sync dock show active boost; `node_health.ps1` / `ibd_convergence_check.ps1` log boost (`node/connect_catchup.go`, `node/ibd_progress_snapshot.go`, `connect_catchup_test.go`, `ui/static/app.js`, `scripts/dogego_rpc.ps1`)
- [x] **MVP:** optional `inv` → `getdata` for missing block payloads (`HandleInvBlockFetch`, capped per message)
- [x] **MVP:** answer peer **`getdata`** for stored blocks and known txs (`HandleInboundGetData`, capped; `notfound` for the rest)
- [x] **MVP:** P2P **witness tx hardening** - reject `tx` with witness (+BIP61 `reject`, misbehavior); ignore `MSG_WITNESS_TX` inv without fetch (`p2p_tx.go`)
- [x] **MVP:** P2P **BIP61 reject on mempool policy failure** - insufficient fee / invalid / non-standard (+ misbehavior for invalid); orphans and duplicate spends silent (`HandleInboundTxAdmissionFailure`, primary + relay + **inv→getdata** tx fetch)
- [x] **MVP:** `CTransaction` wire read/write + BIP141 **size / vsize / weight** in RPC (`wire/txsize.go`, `txToRPCJSON`, mempool vsize); witness **stacks decoded** but **rejected** at mempool/P2P (Dogecoin has no segwit)
- [x] **MVP:** AuxPoW wire decode/encode, `headers_aux.bin`, block parse, serve `getheaders`, validation, mining RPC (`wire/auxpow*.go`, `store/header_aux*.go`, `rpc/auxpow_*.go`)
- [x] **MVP:** AuxPoW parent checks - parent must not be auxpow; parent coinbase single null prevout; merkle branch caps (`checkAuxPow` in `consensus/headers_validate.go`; Core `CAuxPow::check` does not require `CMerkleTx::hashBlock` == parent hash)
- [x] **Core parity:** **legacy coinbase subsidy** - `GetDogecoinBlockSubsidy` pre-145k RNG matches Core (`generateMTRandom` + Boost `uniform_int` division, not `%`; `consensus/mt19937.go`, `subsidy_legacy_vectors_test.go`)
- [x] **MVP:** AuxPoW **chain index range** + parent coinbase size cap + parent **nBits** non-zero (`checkAuxPow`)
- [x] **MVP:** AuxPoW parent **merkle root** non-zero; duplicate orphan txid ignored; `ComputeBlockVersion` for GBT / `createauxblock` (`consensus/versionbits.go`)
- [x] **MVP:** AuxPoW rejects **duplicate chain merkle root** in parent coinbase (`checkAuxPow`)
- [x] **MVP:** AuxPoW parent **nVersion** / **nTime** non-zero (`checkAuxPow`)
- [x] **MVP:** `verifychain` level ≥3 requires **headers_aux.bin** + full auxpow validation for auxpow-version headers (`validate_stored.go`)
- [x] **MVP:** `verifychain` level **4** returns explicit RPC error when **raw blocks** or **tx index** are missing (instead of silent `false`); **verbose** returns failure reason
- [x] **MVP:** **aux block template cache** by payout script + block hash; tip/mempool invalidation (`rpc/auxcache.go`, Core `CAuxBlockCache`)
- [x] **MVP:** AuxPoW **strict chain id** on full blocks (`checkBlockAuxPow`, Core `CheckAuxPowProofOfWork` child header)
- [x] **MVP:** **`submitblock` / generate extend** requires **headers_aux.bin** for auxpow-version headers (`ExtendHeadersFromTipBlock`)
- [x] **MVP:** AuxPoW rejects parent header with **zero prev block hash** (`checkAuxPow`)
- [x] **MVP:** AuxPoW rejects parent **prev hash** or **block id** equal to child block hash (`checkAuxPow`)
- [x] **MVP:** AuxPoW validates parent header **scrypt PoW** against **child `nBits`** (`checkAuxPow`; Core `CheckAuxPowProofOfWork`)
- [x] **MVP:** AuxPoW parent chain id - Core `CAuxPow::check`: reject parent only when chain id equals Dogecoin `0x62` (not “must be zero”; `checkAuxPow`)
- [x] **MVP:** AuxPoW rejects **parent timestamp** more than 2h ahead of child header (`checkAuxPow`)
- [x] **MVP:** AuxPoW validation aligned with Core `CheckAuxPowProofOfWork` + `CAuxPow::check` (parent PoW vs child `nBits`, merge-mining script layout, chain index)
- [x] **Core parity (AuxPoW parent):** embedded parent header + coinbase + merkle branches only - **matches Dogecoin Core** (`CAuxPow::check`, `CheckAuxPowProofOfWork`). **No separate Litecoin chain sync** on Core or DogeGo; optional future parent-chain archive would exceed Core scope.
- [x] **MVP:** AuxPoW parent coinbase **`CheckTransaction`** in `checkAuxPow` (Core-shaped structural validation)

---

## Phase 3 - Headers-first sync (read-only chain)

- [x] **MVP:** `getheaders` → `headers` loop, append to `headers.bin`, prev-hash linkage (`node/fetch.go`, `store/journal.go`)
- [x] **MVP:** interleaved **genesis-only** raw `getdata` during long header catch-up (tip window deferred to post-sync + progressive fetch; avoids single-peer overload)
- [x] **MVP:** inbound `headers` messages after handshake (Core `sendheaders` path) validated and appended (`node/headers_apply.go`, `node/run.go`)
- [x] **MVP:** serve outbound peer **`getheaders`** from local header journal (`store/headers_serve.go`, `wire.EncodeHeadersPayload`)
- [x] **MVP:** per-network chain data under `<datadir>/mainnet/` or `<datadir>/testnet/` (headers, raw blocks, wallet); one-time migrate from legacy flat `headers.bin` when genesis matches (`node/datadir.go`)
- [x] **MVP:** contextual Digishield / auxpow header validation on sync and `verifychain` level ≥3 (`consensus/headers_validate.go`, `validate_stored.go`)
- [x] **MVP:** header reorg truncate + raw/tx prune + UTXO rebuild (`node/headers_apply.go`, `store/reorg_prune.go`)
- [x] **MVP:** **defer header sync** when local headers already match peer height but block bodies lag (Core-style prioritize forward `getdata`; `ShouldDeferHeaderSyncWhileBodiesLag`)
- [x] **MVP:** **contiguous frontier connect** after each batch (`tryConnectContiguousFrontier` - ConnectTip-style from `contiguousTip+1`)
- [x] **MVP:** **multi-peer header top-up** - periodic `getheaders` on ranked **relay** peers (replies applied on each relay read loop; chain-work reorg rules); initial header sync still **failover** across discovery candidates (`node/headers_topup_multi.go`)
- [x] **MVP:** **parallel header assist** at startup - up to 2 extra probed peers run headers-only sync while primary catches up (`node/header_assist_sync.go`); assist **never shares** the primary peer TCP session (avoids `bad magic` desync)
- [x] **MVP:** forward IBD **defers inv block fetch** far ahead of contiguous bodies (`ShouldDeferInvBlockFetch`, `node/ibd.go`)
- [x] **MVP:** **`headers_aux.bin` aligned** to `headers.bin` on truncate and before each header batch (`EnsureRecordCount`, `store/header_aux.go`)
- [x] **MVP:** forward IBD **defers ConnectBlock** on inv/orphan blocks far ahead of contiguous raw frontier (`deferConnectDuringIBD`, `blockstore_coverage.go`)
- [x] **MVP:** **`dogego_sync_health` / `dogego_sync_ok`** on `getblockchaininfo` and web summary - forward IBD active vs stalled (`rpc/sync_display.go`)
- [x] **MVP:** **`dogego_header_sync_recovery`** only for stale tip or live header recovery (not normal headers-ahead-of-bodies IBD); progressive sync can **fetch genesis (height 0)** when missing (`rpc/header_sync_diag.go`, `node/rawsync_progress.go`)
- [x] **MVP:** **`dogego_genesis_missing`** RPC hint; IBD stall recovery **realigns `next_probe_height`** to lowest missing; batched block store failures logged (`node/ibd_stall.go`, `node/rawsync_progress.go`)
- [x] **MVP:** **block-assist** starts once (`EnsureBlockAssistWorkers`); assist workers spin up when peers appear after stall/refresh; dashboard **`lowest_missing_height`** + genesis banner (`node/block_assist_launcher.go`, `ui/summary_build.go`)
- [x] **MVP:** **raw sync reset after chain truncate** (header recovery / `truncatetoheight` clears in-flight getdata and realigns `next_probe_height`; `OnChainTruncated`, `node/rawsync_progress.go`)
- [x] **MVP:** after truncate: **IBD latch reset**, **mempool re-sync**, **assumevalid re-resolve**; no **parallel header assist** when bodies lag headers (`node/header_assist_sync.go`, `rpc/ibd_exit_latch.go`)
- [x] **MVP:** **release in-flight getdata** on primary redial / block-assist disconnect (stale claims no longer stall forward IBD); truncate checkpoint uses **keepThrough+1** (`node/rawsync_progress.go`)
- [x] **MVP:** **`rawblocks_sync.json` checkpoint** tracks contiguous frontier during IBD (`SyncCheckpointToContiguous`)
- [x] **MVP:** batched `headers.bin` / `headers_aux.bin` append (one write+sync per 2000-header batch; `store/journal.go`, `store/header_aux.go`); P2P `ApplyHeadersMessage` uses contiguous wire80 batch (`AppendWireHeaderBatch`) without `[][]byte` slice alloc
- [x] **MVP:** **active fork probe** - before header reorg truncate, `getheaders` at fork height to relay peers + chain-work delta log (`RequestForkProbeFromRelays`, `BuildBlockLocatorFromHeight`)
- [x] **MVP:** **auxpow backfill from raw blocks** - `BackfillAuxThroughHeight` (single `LoadAllRecords` + bounded height scan); startup + every 15 min during IBD capped to `contiguous+2048` when headers far ahead; **near-activation** window around 371337 (`maybeBackfillAuxNearActivation`, `store/header_aux_backfill.go`)
- [x] **MVP:** **inline aux patch on block store** - `PatchRecordAt` (one file read/write per height); skipped for ancient heights during deep IBD (activation window + body frontier only; `store/header_aux_patch.go`, `BlockPutSideband`)
- [x] **MVP:** **bounded aux backfill** - `rewriteRecordsThrough` rewrites prefix `0..through` only (no `LoadAllRecords` of multi-million header tips; `store/header_aux_backfill.go`)
- [x] **MVP:** **`headers_aux.bin` ReadAt / getheaders serve** - per-record file span read + one aux file read per `HeadersAfterFork` batch (`store/header_aux.go`, `store/headers_serve.go`)
- [x] **MVP:** **`verifychain` level ≥3** - `ValidateStoredHeaders` uses one aux snapshot per range; clearer missing-auxpow error; post-attach bounded aux backfill (`consensus/validate_stored.go`, `blockstore_coverage.go`)
- [x] **MVP:** **forward IBD cursor** - `rawblocks_sync.json` may resume at height **0** (genesis); `LowestMissingBlockHeightFrom` scans from contiguous+1 when the frontier is connected; checkpoint sync keeps **next_probe=0** when genesis missing (`store/rawbody_missing.go`, `node/rawsync_progress.go`)
- [x] **MVP:** **operator `truncatetoheight` / invalidate** - `OnChainTruncated` restarts block-assist + background header sync (`node/run.go`, `node/chain_truncate.go`)
- [x] **MVP:** **IBD stall recovery** calls DNS/fixed-seed **`RefreshDiscovery`** when block download stalls (`MaybeRecoverIBDStall`, `node/ibd_stall.go`)
- [x] **MVP:** **header sync peer probe** - parallel handshake (up to **6** candidates, **8** workers, **48** dial cap), DNS seeds before score-file walk, prefer highest **start height**; **DialableOrder** skips short dial cooldowns before scored dead peers (`probeHeaderSyncPeers`, `HeaderSyncProbeCandidates`, `BlockPeerScorer.DialableOrder`)
- [x] **MVP:** block-fetch **peer scoring** for `notfound` / batch-missing (`sessionFailureHardFromFetchErr`)
- [x] **MVP:** **marginal chain-work reorg defer** - fork with <5% work advantage deferred (fork probe + await longer chain) unless `preciousblock`
- [x] **MVP:** **multi-peer best-chain election** - before header reorg truncate, sync `getheaders` probe to up to 2 relay peers; reject incoming fork when alternate branch chain work wins (`header_chain_election.go`)
- [x] **MVP:** **stale header time rewind** - inbound batch `nTime` far ahead of local tip → truncate to last difficulty period + retry `getheaders` (`node/headers_stale_rewind.go`)
- [x] **MVP:** **torn `headers.bin` repair** on open (drop partial trailing record after force-kill; `store/journal.go`)
- [x] **MVP:** **header segment storage** - `headers/seg/NNNNNNNNNN.bin` (2000 headers/segment), `headers/manifest.json`, atomic `.tmp` writes, one-time migrate from `headers.bin` → `.legacy` (`store/header_segments.go`, `store/header_chain.go`, `OpenHeaderChain`)
- [x] **MVP:** **`headers_sync.json` checkpoint** - atomic crash checkpoint after each header batch; monolith + segment repair from checkpoint on startup (`store/header_checkpoint.go`)
- [x] **Core parity:** **single dedicated header IBD owner** - dedicated peer owns header catch-up; background sync only on stall/yield/watchdog; no parallel header assist at boot (`node/header_sync_dedicated.go`, `header_sync_coord.go`, `header_sync_stall.go`)
- [x] **Core parity:** **header catch-up during IBD** - no false “caught up” when peer height ≈ local tip after one batch; empty headers from far-ahead peer is error (`header_sync_dedicated.go`, `fetch.go`, `shouldContinueHeaderCatchUpDuringIBD`)
- [x] **Core parity:** **local genesis from chainparams** - `EnsureLocalGenesis` stores Core-shaped genesis block when headers exist but `rawblocks/` lacks height 0; validates journal genesis matches chainparams (`node/genesis_local.go`, `chain/genesis_block.go`)
- [x] **Core parity:** **genesis / ancient block peer rotation** - primary redial when pruned peer returns notfound for genesis or heights ≤512 (`ErrGenesisPeerNotFound`, `shouldRedialPrimaryForAncientFetch`, `run.go`)
- [x] **Core parity:** **block peer score persistence (archival)** - `block_peer_scores.json` v2 stores NODE_* services + start height; discovery/assist merge deprioritizes pruned peers for ancient heights (`block_peer_score_persist.go`, `RefreshBlockAssistPool`)
- [x] **Core parity:** **IBD stall recovery (genesis-aware)** - 2 min stall interval while genesis body missing; stall sweep calls `EnsureLocalGenesis` + archival assist refresh (`ibd_stall.go`, `ibd_stall_genesis_test.go`)
- [x] **Core parity:** **body IBD owns pipeline when headers far ahead** - `reconcileHeaderCatchUpPending`, `ShouldPauseHeaderCatchUpForBodyIBD`; unknown peer height no longer latches header catch-up at startup (`node/ibd_header_body_priority.go`, `header_sync_stall.go`)
- [x] **Core parity:** **block-assist seed before DNS** - fixed peers + score file seed assist pool synchronously at startup (`node/block_assist_seed.go`, `run.go`)
- [x] **Core parity:** **raw sync checkpoint resume** - `initProgressiveRawAtStartup` after recovery sweeps, before early RPC; realigns `rawblocks_sync.json` to contiguous+1 (`node/rawsync_progress.go`, `run.go`)
- [x] **Core parity:** **dashboard live feed during IBD** - rate-limited raw block count reconcile, P2P snapshot timeout/cache, bootstrap summary when build blocks (`ui/live.go`, `ui/raw_count_reconcile.go`, `ui/p2p_snapshot_timeout.go`)
- [x] **Core parity:** **dashboard/RPC header tip from disk** - `SyncTipFromDisk` + `journalTipForDashboard` before `/api/summary`; segment-mode `LastTipHash` / `BestBlockHashHex` (`ui/summary_build.go`, `store/journal.go`)
- [x] **Core parity:** **header checkpoint hashes** - Core `mapCheckpoints` enforced at listed heights during `ValidateHeaders` (`chain/checkpoints.go`, `consensus/header_checkpoints.go`; `checkpoints` in `dogecoinconf.json`, default on)
- [x] **Core parity:** **nMinimumChainWork** - mainnet minimum chain work (block 5,050,000) gates `initialblockdownload` like Core `IsInitialBlockDownload` (`chain/minimum_work.go`, `rpc/chaininfo_helpers.go`)
- [x] **Core parity:** **-maxtipage** - stale chainActive tip time keeps `initialblockdownload` true (`chain/maxtipage.go`, `maxtipage` in config / CLI; `rpc/chaininfo_helpers.go`)
- [x] **Core parity:** **GuessVerificationProgress** - `dogego_tx_verification_progress` on `getblockchaininfo` (mainnet tx curve; `rpc/sync_display.go`, tx index file count)
- [x] **Core parity:** **getchaintips** `valid-headers` when partial raw bodies exist ahead of chainActive (`rpc/chaintips.go`)
- [x] **Core parity:** **IBD exit latch** - `initialblockdownload` stays false after first catch-up like Core `IsInitialBlockDownload` (`rpc/ibd_exit_latch.go`)
- [x] **Core parity:** **`getaddressinfo`** / **`listreceivedbylabel`** wired in JSON-RPC dispatch (were listed in help but fell through to not implemented)
- [x] **Core parity:** **network time for header validation** - `ValidateHeaders` uses Core **GetTime** (median peer `timeoffset`, else sync-peer version `nTime`; `clock/network_time.go`, `node/network_time.go`, `BlockStoreCtx.NetworkTimeUnix`)
- [x] **Core parity:** **`verifychain`** level ≥3 uses network-adjusted time (same as live header sync; `rpc/verifychain.go`)
- [x] **Core parity:** **`verifychain`** mock-journal fallback uses RelaxedPoW linkage when no `*store.HeaderJournal`; level-4 integration uses `chain.RebootTestnetGenesisBlockRaw()` (`rpc/verifychain.go`, `verifychain_level4_integration_test.go`)
- [x] **Core parity:** **`getinfo`** / **`getmininginfo`** expose `headers`, `initialblockdownload`, `verificationprogress` aligned with `getblockchaininfo` (`rpc/chaininfo_helpers.go`)
- [x] **Core parity:** **`getblockchaininfo.blocks`** = **chainActive** (UTXO/connect tip), not orphan stored bodies ahead of `ConnectBlock` (`rpc/chaininfo_helpers.go`)
- [x] **Core parity:** Web UI `/api/summary` and P2P status use `ComputeChainIBDSnapshot` (min chain work, `-maxtipage`, IBD latch, tx verification curve) - `ui/server.go`, `node/ibd_status.go`, `node/run.go`; P2P snapshot exposes **`chain_active_height`**
- [x] **Core parity:** Web UI exposes `dogego_tx_verification_progress` / `dogego_body_verification_progress` / `dogego_connected_verification_progress`; dashboard shows **connected** (chainActive) vs **stored bodies** vs headers (`ui/static/app.js`, `getblockchaininfo`)
- [x] **Core parity:** **`verificationprogress`** during IBD uses **chainActive ÷ headers** when the header tip is ahead of connect (`ibdProgress` in `rpc/chaininfo_helpers.go`); operator runbook updated (`docs/CORE_OPERATOR_RUNBOOK.md`)
- [x] **Core parity:** wallet rescan/import height caps and **`/api/chainstats`** use **chainActive** (UTXO tip via `ActiveChainBlockHeight(..., paths)`), not stored bodies ahead of connect (`rpc/wallet_*.go`, `ui/chainstats.go`, `ui/chainstats_hints`)
- [x] **Core parity:** early IBD header sync - no false **compressed-period** rewind mid-retarget window; no **parallel header assist** below height 100k; defer UTXO rebuild on truncate when bodies lag (`headers_stale_rewind.go`, `header_assist_sync.go`, `chain_truncate.go`)
- [x] **Core parity:** **automatic header stall recovery** - stall timeout on silent peers, background catch-up when inline sync yields for block IBD, header advance watchdog, IBD header top-up on primary (`header_sync_stall.go`, `fetch.go`, `run.go`)
- [x] **Core parity:** **`submitblock`** header extend uses median-adjusted network time (`rpc/submitblock.go`)
- [x] **Core parity:** **`decodescript` asm** - standard opcode names aligned with Core `GetOpName` (`wire/script_opnames.go`)
- [x] **MVP:** reject **regressed `nTime`** when a header batch continues a stale local tip (`consensus/headers_validate.go`)
- [x] **Core parity:** **adequate raw body** checks end-to-end - fetch/store/contiguous/inv paths ignore undersized `rawblocks/` stubs; genesis fetch verifies payload size (`store/rawbody_adequate.go`, `node/fetch.go`, `node/blockstore.go`)
- [x] **Core parity:** **startup purge** of undersized raw block stubs + reset contiguous tip (`store/rawbody_purge.go`, `node/run.go`)
- [x] **Core parity:** **sync phase / status line** - `DogeGoSyncPhase` + `SyncStatusLine` use contiguous bodies (not chainActive alone); UI/RPC no false “Up to date” during body IBD (`rpc/sync_phase.go`, `ui/summary_build.go`, `ui/static/app.js`)
- [x] **Core parity:** **IPv4-first peer dial** - DNS discovery and `DialableOrder` try IPv4 before IPv6 (reduces Windows `connectex: unreachable network` on v6); shuffle within each group (`p2p/addrorder.go`, `p2p/discover.go`, `node/block_peer_score.go`)
- [x] **Core parity:** **post-rewind stub purge** - `truncatetoheight` / header rewind removes inadequate `rawblocks/` stubs (`node/chain_truncate.go`, `BlockStoreCtx.PurgeInadequateRawBodies`)
- [x] **Core parity:** **`MAX_BLOCKS_IN_TRANSIT_PER_PEER`** - progressive getdata batches default to **16** blocks per peer (cap **32** with many lanes); primary idle loop runs up to **3** batches per read timeout like header interleave (`node/ibd.go`, `tryFetchMissingBatches`)
- [x] **Core parity:** **`PrepareAtStartup` at genesis tip** - block sync arms when header journal is only height **0** (genesis body fetch), not only when `tip >= 1` (`node/rawsync_progress.go`)
- [x] **Core parity:** **claimBatch / cursor at height 0** - forward IBD claims genesis when `tip == 0`; checkpoint init and stall realign allow tip **0** (`node/rawsync_progress.go`)
- [x] **Core parity:** **block-assist + relay IBD throughput** - up to **2** progressive getdata batches per assist/relay idle round (primary already uses **3**) (`node/block_assist.go`, `node/relay_session.go`)
- [x] **Core parity:** **`pruneblockchain` marker** - `prune_marker.json` sets `getblockchaininfo` **`pruned`** / **`prune_height`** after operator prune (`store/prune_marker.go`, `rpc/chain_control.go`)
- [x] **Core parity:** **`pruneblockchain` RPC integration** - removes raw bodies below height while keeping headers; chainActive cap via `ContiguousRawHeight` (`rpc/pruneblockchain_integration_test.go`)
- [x] **Core parity:** **inbound getdata tx from tx index** - confirmed txs served from `indexes/tx` + raw block offset (`TestHandleInboundGetData_TxFromIndex`)
- [x] **Core parity:** **inbound serve black-box (tx index + notfound)** - `TestInboundServeBlackBoxConfirmedTxAndNotFound` (`node/inbound_serve_blackbox_test.go`)
- [x] **Core parity:** **operator RPC error codes (golden)** - Core-shaped `-32602`/`-8`/`-18`/`-25`/`-26` for prune, truncate, getblock, sendraw (`rpc/operator_rpc_errors_test.go`)
- [x] **Core parity:** **getheaders hashStop + max cap** - `HeadersAfterFork` stops before hashStop; per-message cap (`store/headers_serve_test.go`, `node/getheaders_serve_test.go`)
- [x] **Core parity:** **prioritisetransaction latent mapDeltas RPC** - accept fee_delta before mempool admit; `getmempoolentry` modifiedfee (`TestExecPrioritiseTransactionLatentGetMempoolEntry`; fixed ghost-txid latent test in `net_control_test.go`)
- [x] **Core parity:** **reindexblockfilters idempotency** - `TestExecReindexBlockFiltersIdempotentSecondPass`
- [x] **Core parity:** **verifychain after pruneblockchain** - level-2 header linkage on retained range + `getblock` at tip (`TestExecVerifyChainAfterPrune`)
- [x] **Core parity:** **savemempool/loadmempool fee_deltas RPC** - latent + in-pool mapDeltas round-trip (`TestExecSaveLoadMempoolFeeDeltasRoundTrip`)
- [x] **Core parity:** **inbound getdata batch cap** - `MAX_BLOCKS_IN_TRANSIT`-shaped 16-block serve limit (`TestHandleInboundGetDataBlockBatchCap`)
- [x] **Core parity:** **inbound getdata tx batch cap** - 8-tx serve limit + `notfound` for remainder (`TestHandleInboundGetData_TxBatchCap`)
- [x] **Core parity:** **inbound getdata mixed batch cap** - independent 16-block + 8-tx limits in one `getdata` (`TestHandleInboundGetData_MixedBatchCap`)
- [x] **Core parity:** **inbound getdata witness inv** - `MSG_WITNESS_TX` → `notfound` (full tx only) (`TestHandleInboundGetData_WitnessInvNotFound`)
- [x] **Core parity:** **prepareHeadersForConnect chain election** - `BlockStoreCtx.chainElection` rejects fork before truncate (`TestPrepareHeadersForConnect_chainElectionReject`)
- [x] **Core parity:** **prepareHeadersForConnect preciousblock** - low-work fork accepted when batch contains precious hash (`TestPrepareHeadersForConnect_preciousOverridesLowWork`)
- [x] **Core parity:** **prepareHeadersForConnect marginal reorg defer** - tiny chain-work advantage deferred (Core hair-trigger guard) (`TestPrepareHeadersForConnect_marginalReorgDeferred`)
- [x] **Core parity:** **prepareHeadersForConnect fork probe** - `SetForkProbe` fires before reorg truncate (`TestPrepareHeadersForConnect_forkProbeCalled`)
- [x] **Core parity:** **ApplyHeadersMessage invalid block policy** - rejects headers marked invalid via `chain_policy.json` (`TestApplyHeadersMessage_rejectsInvalidMarkedBlock`)
- [x] **Core parity:** **inbound getdata adversarial matrix** - empty payload, malformed wire, unknown inv → notfound, witness block serves full block, filtered block → notfound, duplicate block inv serves twice (`getdata_serve_test.go`, `inbound_serve_blackbox_test.go`)
- [x] **Core parity:** **inbound BIP157 filter serve** - getcfilters/cfheaders serve + unknown stop hash + range cap + malformed wire (`node/filter_serve_test.go`)
- [x] **Core parity:** **inbound getheaders adversarial** - malformed wire, unknown locator falls back to genesis fork (`getheaders_serve_test.go`)
- [x] **Core parity:** **inbound getheaders empty at tip** - zero-header reply when locator is chain tip (`TestHandleInboundGetHeadersEmptyAtTip`)
- [x] **Core parity:** **inbound getheaders max cap** - P2P reply capped at Core `MAX_HEADERS_RESULTS` (2000) (`TestHandleInboundGetHeadersMaxCap`)
- [x] **Core parity:** **truncatetoheight idempotency** - repeated operator truncate succeeds (`TestExecTruncateToHeightIdempotentSecondCall`)
- [x] **Core parity:** **operator RPC golden errors (extended)** - … + batch-72 aux/multisig/account arity paths (`operator_rpc_errors_test.go`; **1560** subtests; `TestOperatorRPCGoldenSubtestCount`; batch-71 chain-control/wallet-list; batch-70 psbt/mining/wallet-process; batch-69 sendmany/importmulti/wallet-flag; batch-68 backup/bumpfee/sendfrom; batch-67 wallet/stub/psbt; batch-66 derive/import/aux; batch-65 generate/submitpackage/import/descriptor; batch-64 send/psbt/wallet-label; batch-63 wallet/encryption/fee; batch-62 wallet/import/sign; batch-61 wallet/balance/psbt/template; batch-60 aux/mining/mempool/psbt/accounts; batch-59 wallet-received/labels/lockunspent/tip-wait/sendraw/aux; batch-58 wallet-psbt/mining/star-account/blockfilter-arity; batch-57 wallet/net/psbt-arity/prioritise; batch-56 psbt/raw-tx/submitblock/wallet-arity/star-account; batch-55 blockhash-type/mempool-arity/wallet-arity/sendfrom; batch-54 move/psbt/wallet-arity/blockfilter-type; batch-53 wallet-empty/scan/mempool/fundraw/sendfrom; batch-52 blockfilter/raw-tx/wallet-empty/maintenance; batch-51 chain-control hash/mempool/wallet-raw-tx; batch-50 chain-control/scan/scantxoutset/wallet-backup; prune idempotency in `pruneblockchain_integration_test.go`)
- [x] **Core parity:** **pruneblockchain idempotency** - repeated prune at same height returns 0 removed (`TestExecPruneBlockchainIdempotentSecondCall`)
- [x] **Core parity:** **upgradetxindex idempotency** - repeated upgrade on empty index (`TestExecUpgradeTxIndexIdempotentSecondPass`)
- [x] **Core parity:** **savemempool idempotency** - repeated `savemempool` succeeds (`TestExecSaveMempoolIdempotentSecondSave`)
- [x] **Core parity:** **loadmempool idempotency** - repeated `loadmempool` from same file (`TestExecLoadMempoolIdempotentSecondLoad`)
- [x] **Core parity:** **wallet listtransactions paging** - offset/limit slice (`TestWalletListTransactionsPageOffsetLimit`)
- [x] **Core parity:** **wallet listtransactions scan-index fast path** - skip UTXO receive walk when **`wallet.db`** has receive rows; **`GET /api/wallet/txs?type=all`** merged fallback uses scan history (`rpc/wallet_rpc.go`, `ui/wallet_fast.go`, `TestExecListTransactionsWalletManyUtxosUsesScanIndex`); **`dogego_wallet_scan_index_ok`** / **`dogego_wallet_history_fast_path`** / **`dogego_wallet_listtransactions_utxo_walk`** / **`dogego_wallet_listtransactions_scan_pending`** on **`getwalletinfo`** + matching web fields on **`GET /api/wallet`**, **`GET /api/summary`**, and **`GET /api/core-wallet-probe`**; dashboard auto-rescan when **`needs_rescan`** or **`wallet_listtransactions_utxo_walk`** with >64 UTXOs; History tab defers heavy **`listtransactions`** fetch while rescan builds index (`ui/static/app.js`); **`GET /api/wallet/txs`** + **`/api/wallet/txs.csv`** return **`deferred`** + **`defer_reason`** during IBD/connect lag/scan build (`ui/wallet_tx_defer.go`); **`GET /api/wallet`** + **`GET /api/summary`** expose **`wallet_history_deferred`** + **`wallet_history_defer_reason`**; **`getwalletinfo`** exposes **`dogego_wallet_history_deferred`** + **`dogego_wallet_history_defer_reason`** (`wallet/history_defer.go`); **`GET /api/core-wallet-probe`** exposes **`wallet_history_defer_reason`** and skips **`listtransactions`** latency gate when deferred (mirrors dashboard + `core_wallet_workflow.ps1`)
- [x] **Web UI:** **wallet History CSV HTTP** - `TestWalletTxsCSVEndpoint`
- [x] **Web UI:** **wallet History HTTP pagination** - `registerWalletTxsRoutes` + `TestWalletTxsHTTPPagination` + large-limit first-page cap (`TestWalletTxsHTTPFirstPageLargeLimit`) + partial-page meta for Load-all UI (`TestWalletTxsHTTPPartialPageMeta`)
- [x] **Core parity:** **`BLOCK_DOWNLOAD_WINDOW` (1024)** - frontier-first forward IBD caps getdata stripes at `lowMissing+1023` (Core `validation.h`); parallel lanes still use the wider `forwardIBDParallelWindow` when striping (`node/ibd.go`)
- [x] **Core parity:** **`BodiesBehindHeaders` at genesis-only tip** - header journal height **0** with missing genesis body reports bodies-behind (arms block sync / defers header pull) (`node/ibd.go`, `node/ibd_test.go`)
- [x] **Core parity:** **`dogego_headers_sync_progress`** on `getblockchaininfo` when headers run ahead of chainActive (`rpc/sync_phase.go`, `rpc/dispatch.go`)
- [x] **Core parity:** **block download timeout** - batched getdata read deadline uses Core `BLOCK_DOWNLOAD_TIMEOUT_BASE` + `PER_PEER` scaled by parallel sync lanes; per-lane in-flight batch window disconnects slow peers (`node/block_download_timeout.go`, `block_download_timeout_peer.go`, `fetch.go`, `rawblocks_sync.json` snapshot `block_download_timeout_sec`)
- [x] **Core parity:** **headers download timeout** - stale header tip (>24h behind network time) uses Core `HEADERS_DOWNLOAD_TIMEOUT_*` formula (`node/header_sync_stall.go`, `fetch.go`)
- [x] **Core parity:** **header-sync peer cooldown** - stall/timeout header peers get `NoteSessionFailure` when rotating to next candidate (`node/header_sync_peer_score.go`, background/dedicated/configured header sync)
- [x] **Core parity:** **IBD stall diagnostics on `getblockchaininfo`** - top-level `dogego_last_block_stall_*`, `dogego_last_block_download_timeout_*`, `dogego_block_*_timeout_sec` from `dogego_raw_sync` (`rpc/sync_display.go`)
- [x] **Core parity:** **Web Overview stall hints** - `/api/summary` exposes last stall/timeout peer; sync activity logs peer disconnects; parallel header-assist peers down-ranked on failure (`ui/summary_build.go`, `ui/static/app.js`, `node/sync_activity.go`, `header_assist_sync.go`)
- [x] **Core parity:** **per-lane in-flight block cap** - no new getdata claims when a sync lane already holds `MAX_BLOCKS_IN_TRANSIT_PER_PEER` heights (`scanClaimRange`, `inFlightCountForLaneLocked`)
- [x] **Core parity:** **fetch failures update addrbook** - hard block/header stall/timeout penalties call `AddrBook.NoteFailure` in addition to block peer scorer (`node/peer_penalty.go`)
- [x] **Core parity:** **`BLOCK_STALLING_TIMEOUT` (2s)** - frontier in-flight height releases claim, disconnects stall peer (primary redial / relay disconnect / assist exit), and cools down via block peer scorer (`node/block_stall.go`, `ErrBlockDownloadStall`, `tryFetchMissingBatches`, `MaybeRecoverIBDStall`)
- [x] **Core parity:** **addrbook tried cap on success** - `NoteSuccess` enforces tried table size; demotes **oldest LastSeen** tried entry (not insertion order) (`node/addrbook.go`)
- [x] **Core parity:** **`getnodeaddresses` RPC** - returns addrbook rows with `time` / `services` / `address` / `port` / `network` (`rpc/net_nodeaddresses.go`, `DataPaths.NodeAddresses`)
- [x] **Core parity:** **header peer score on dedicated sync** - `DownloadHeaders` credits `NoteHeadersDelivered` per validated batch (primary, dedicated, background recovery, header assist; relay inbound already scored)
- [x] **Core parity:** **hard block-fetch penalties on all lanes** - primary, relay idle-fetch, and block-assist use `penalizeBlockPeer` (scorer + addrbook on hard errors)
- [x] **Core parity:** **addnode → tried table** - persistent `addnode` entries seeded as tried on startup (`AddrBook.NoteAddnodePersistent`, `SeedPeerMgr`)
- [x] **Core parity:** **IBD in-flight diagnostics** - `dogego_raw_sync` exposes `max_blocks_in_transit_per_peer` and per-lane `lane_in_flight` on RPC/UI (`rawsync_progress.go`, `rpc/sync_display.go`, `ui/summary_build.go`)
- [x] **Core parity:** **getaddr / getnodeaddresses /16 spread** - `AddrSample` and `NodeAddressRows` prefer one address per IPv4/IPv6 /16 group when possible (`addrbook.go`)
- [x] **Core parity:** **addrbook LastSeen on block delivery** - `NotePeerBlock` → `TouchSeen` keeps tried-table LRU accurate (`peermgr.go`, `addrbook.go`)
- [x] **Core parity:** **IBD stall recovery addrbook penalty** - `MaybeRecoverIBDStall` passes addrbook into frontier stall disconnect (`ibd_stall.go`)
- [x] **Core parity:** **Web Overview in-flight hint** - stalled/active forward IBD shows per-peer in-flight getdata counts (`ui/static/app.js`)
- [x] **Core parity:** **outbound dial /16 diversity (tried + new)** - `dialScoreLocked` penalizes crowded /16 in both tables; `PickBest` skips groups already used by connected sessions (`addrbook.go`)
- [x] **Core parity:** **header-sync probe /16 spread** - `HeaderSyncProbeCandidates` round-robins DNS and score-file tails by /16 before IPv4-first shuffle (`SpreadHostPortsByGroup16`, `header_peer_pick.go`)
- [x] **Core parity:** **block-assist dial /16 spread** - assist pool keeps **addnode** first, round-robins DNS/learned tails by /16, then IPv4-first (`block_assist_candidates.go`, `assistPeerCandidates`)
- [x] **Core parity:** **addrbook on outbound handshake** - block-assist and header-sync probe call `NoteTry` / `NoteSuccess` / `NoteFailure`; stall/timeout + hard fetch penalties on **all lanes** pass addrbook (`RecordOutboundDialTry`, `block_assist.go`, `header_peer_pick.go`, `fetch.go` interleaved primary getdata)
- [x] **Core parity:** **startup header probe addrbook** - load `learned_addrs.json` before inline probe, persist probe handshakes, adopt same book into `PeerMgr` (`bootstrapAddrBook`, `MaybeSaveAddrBookIfDirty`, `activeAddrBook`)
- [x] **Core parity:** **configured peer + primary redial addrbook** - `-peer` / `RedialPrimary` record try/success/failure; `ReplacePrimary` marks new primary tried (`run.go`, `primary_reconnect.go`)
- [x] **Core parity:** **scorer merge discovery /16 spread** - `MergeDiscoveryCandidates` round-robins fresh DNS/learned tail by /16 (`block_peer_score_persist.go`)
- [x] **Core parity:** **relay outbound dial addrbook** - `tryDialMore` uses shared `RecordOutboundDialTry` / `RecordOutboundHandshakeResult` (`peermgr.go`)
- [x] **Core parity:** **`addnode onetry` addrbook** - `DialOnce` records try/success/failure and persists `learned_addrs.json` on attach (`peermgr.go`)
- [x] **Core parity:** **fork-election header probe addrbook** - pre-reorg `getheaders` probes update try/success/failure (`header_chain_election.go`)
- [x] **Core parity:** **block-assist archival peer preference** - record NODE_* / start height from handshake; prefer full NODE_NETWORK peers for ancient block fetch (BIP159 limited peers deprioritized) (`node/block_peer_archival.go`, `chain/p2p_services.go`, `block_assist.go`)
- [x] **Core parity:** **primary block peer selection** - startup probe picks archival NODE_NETWORK peer for getdata (not just 2nd-highest height); redial uses same policy (`pickBlockPrimaryPeer`, `primary_reconnect.go`, `run.go`)
- [x] **Core parity:** **connect-body-gap IBD realign** - when ConnectTip needs a missing body below the download cursor, purge unreadable bundled locators, realign `next_probe_height` to `chainActive+1`, and prioritize that height in `claimBatch` / inv defer (`ConnectBodyGapHeight`, `LowestMissingForIBD`, `node_health.ps1`)
- [x] **Core parity:** **UTXO-snapshot-ahead body replay** - `ConnectFrontierHeight`, deferred connect, monotonic contiguous (`shrinkContiguousTipAfterBodyRemoved`, sequential `noteBlockStoredAt`), targeted purge; `node_health.ps1` snapshot replay line
- [x] **Core parity:** **Core-style body IBD pump** - proactive getdata every **1.5s** on primary + relay + **block-assist** lanes during forward body IBD (not only P2P read idle); `ensureBodyDownloadArmed` clears stale `idleFull`; stall recovery runs whenever bodies lag headers; block-assist session rotate (**45s**) + worker relaunch (**90s**); **`MaybeResumeHeaderCatchUpAfterBodyIBD`** re-arms header sync when body pause lifts (`node/ibd_header_body_priority.go`, `node/ibd_body_pump.go`, `node/ibd_stall.go`, `node/relay_session.go`, `node/block_assist.go`)

---

## Phase 4 - Consensus validation (headers + blocks)

- [x] **MVP:** Header prev-link validation; `RelaxedPoW` flag for tests only (`false` on mainnet and reboot testnet); scrypt PoW when `RelaxedPoW=false` (`consensus/header.go`, `chain/params.go`)
- [x] **MVP:** P2PKH / P2PK / P2SH script verification on mempool admission and `ConnectBlock` (`consensus/script_verify.go`, `p2pk.go`)
- [x] **MVP:** SegWit **implemented, disabled** like Dogecoin Core (`consensus/witness_policy.go` - `IsWitnessEnabled` always false; BIP9 `Timeout: 0`; see [docs/SEGWIT_STATUS.md](docs/SEGWIT_STATUS.md))
- [x] **MVP:** **bare multisig** scriptPubKey verification on connect/mempool (`verifyInputBareMultisig`, `consensus/script_verify.go`)
- [x] **MVP:** Script push decoding for **PUSHDATA1/2/4** and OP_1..OP_16 in scriptSig / P2SH redeem (`consensus/script_push.go`; fixes large DER signatures)
- [x] **MVP:** **BIP9-aware script flags** at `ConnectBlock` / mempool (`ScriptFlagsForChain` + header journal for CSV activation; CLTV/CSV redeem locktime uses `ReadScriptNumPush`)
- [x] **MVP:** **P2SH CLTV/CSV + multisig** redeem verification (`consensus/timelock_multisig.go`); multisig pubkey parsing via `ReadScriptPush` (`consensus/multisig.go`)
- [x] **MVP:** **P2SH P2PK** redeem verification; P2PK pubkey push via `ReadScriptPush` (`consensus/p2pk.go`, `script_verify.go`)
- [x] **MVP:** unified **P2SH CLTV/CSV** redeem path for inner **P2PKH / P2PK / multisig** (`consensus/timelock_redeem.go`); `decodescript` **`dogego_script_template`** metadata (`redeem_meta.go`)
- [x] **MVP:** **nested P2SH** (HASH160 forward redeem → inner P2PKH/P2PK/multisig/timelock) in `consensus/p2sh_nested.go`; **recursion depth cap** (`MaxP2SHRedeemDepth`)
- [x] **MVP:** `CheckBlockDuplicateTxids` on `CheckBlock` (`consensus/block_check.go`)
- [x] **MVP:** `signrawtransaction` P2SH (`redeemScript` / **`innerRedeemScript`** for nested forward + CLTV/CSV inner) + bare P2PK / multisig (`consensus/signing_target.go`, `p2sh_nested_test.go`, `signrawtx_nested_cltv_test.go`)
- [x] **MVP:** `CheckTransaction` rejects **empty scriptPubKey** and **non-zero OP_RETURN** outputs (`bad-txns-unspendable-output`); `decoderawtransaction` / `getblock` `scriptSig.dogego_redeem` + **`dogego_redeem_pushes`** (nested/multisig redeems)
- [x] **MVP:** `verifychain` **verbose** third param + explicit error when **headers_aux.bin** missing for auxpow headers in range; failures logged via **`applog`** when `verbose=false`
- [x] **MVP:** `verifychain` level **4** uses in-memory **UTXO cache** for `ConnectBlock` prevouts when cache tip covers the verified range (`ConnectBlockPrevOutView`)
- [x] **MVP:** **`-assumevalid`** (Core default on mainnet height **5,050,000**) - skip ECDSA/script verification on buried blocks in the best chain; tip window (~**20,160** blocks) still fully verified; `verifychain` forces full scripts (`consensus/assumevalid.go`, `dogecoinconf.json` / `-assumevalid`; `0` = verify all)
- [x] **MVP:** **bulk IBD connect deferral** below assume-valid height - throttle `ConnectBlock` / frontier connect during forward download; flush on catch-up (`node/blockstore_coverage.go`); **never defer below height 4096** (genesis window coinbase connect)
- [x] **Core parity:** **coinbase subsidy cap** at block store - `CheckBlockCoinbaseSubsidyPayload` before `Raw.Put` (Core `ConnectBlock` rule; legacy bug hint on mainnet)
- [x] **Core parity:** **UTXO replay via ConnectBlock** - `SyncUtxoCache` / `RebuildUtxoThrough` (no `ApplyBlockRaw`-only catch-up); truncate rebuild uses connect path
- [x] **Core parity:** **no peer misbehavior** for local legacy-subsidy `bad-cb-amount` (`reject_peer.go`)
- [x] **MVP:** Legacy sighash **OP_CODESEPARATOR** stripping preserves PUSHDATA2/4 pushes (`wire/sighash.go`)
- [x] **Core parity:** mempool standard policy enforces **BIP147 NULLDUMMY** on multisig spends (`ScriptVerifyNullDummy` in `ScriptFlagsForMempool`; connect-block path unchanged - Dogecoin has no segwit)
- [x] **Core parity:** mempool **DISCOURAGE_UPGRADABLE_NOPS** - rejects OP_NOP1/4-10 and CLTV/CSV when not active at spend height (`consensus/discourage_nops.go`, `ScriptFlagsForMempool`)
- [x] **Core parity:** **witness program outputs non-standard** when segwit disabled - BIP141 `OP_0`/`OP_1..16` templates classified and rejected in `IsStandardScript` (`consensus/witness_program.go`; matches Core `IsStandard` with `witnessEnabled=false`)
- [x] **Core parity:** **`getblockchaininfo` header sync diagnostics** - `dogego_header_tip_time`, tip age vs `-maxtipage`, headers-ahead-of-chainActive, recovery hints (`rpc/header_sync_diag.go`)
- [x] **Core parity:** **Web `/api/summary`** merges same header sync diagnostics + overview banner when `dogego_header_sync_recovery` is set (`ui/server.go`, `node/run.go`)
- [x] **Core parity:** **header journal recovery** - startup detect compressed retarget-window `nTime`; rewind on peer `nTime` jump, `bad nBits` at retarget, or tight period span (`node/headers_stale_rewind.go`, `headers_apply.go`)
- [x] **Core parity:** **`dogego_recoverheaders`** + **`truncatetoheight`** operator RPCs; web **`POST /api/chain/recover-headers`** on Overview when `dogego_header_sync_recovery` is set (`node/recover_headers.go`, `rpc/dogego_recoverheaders.go`, `ui/server.go`)
- [x] **Core parity:** Web **Console JSON-RPC** - loopback **`POST /api/rpc`** (in-process `rpc.Dispatch`); presets for chain/network/mining/header recovery (`ui/server.go`, `ui/static/app.js`)
- [x] **Core parity:** PoW retarget at mainnet **#1920** (240-block window; Core `pow_tests` #2015 uses Bitcoin 2016-block interval - not applicable) (`consensus/difficulty_test.go`)
- [x] **Core parity:** **solo-primary `getpeerinfo`** - single-TCP path exposes `relaytxes`, `minping`/`pingwait`, `banscore`, per-msg byte maps, `last_block`/`last_transaction`, `inflight` (`node/peerinfo_solo.go`, `run.go`)
- [x] **Core parity:** **`getnetworkinfo.networks[].limited`** - ipv4 `limited: true` when inbound listen is disabled (`node/networks_rpc.go`)
- [x] **MVP:** BIP9 version-bits state for `getblockchaininfo` bip9_softforks (csv, segwit; `consensus/versionbits.go`)
- [x] **MVP:** **`getdeploymentinfo`** - buried BIP34/65/66 + BIP9 deployment state at tip or block hash (`rpc/deploymentinfo.go`)
- [x] **MVP:** BIP9 **period statistics** (`statistics` in `getdeploymentinfo` / `bip9_softforks` when deployment `started`; `consensus/BIP9PeriodStatsAt`)
- [x] **MVP:** **`ComputeBlockVersion`** for mining templates (BIP9 started/locked-in bits + auxpow chain id; `getblocktemplate`, `createauxblock`)
- [x] **MVP:** **`getblocktemplate`** Core fields - `coinbaseaux`, `vbavailable` / `vbrequired`, BIP9 `rules` (`consensus/GBTVersionBits`, `rpc/getblocktemplate.go`)
- [x] **MVP:** GBT version-bits aligned with **Dogecoin Core** - `gbt_vb_name` (`!` when not forced), `vbrequired: 0`, client `rules` support check for active deployments
- [x] **MVP:** GBT **`version`** masks unsupported STARTED bits (`GBTBlockVersion`); legacy **`maxversion`** cap + **`version/force`** mutable when `rules` omitted (Core mining.cpp)
- [x] **MVP:** GBT **`bits`/`target`** from Digishield **`NextBlockBits`** (not tip `nBits`); BIP22 **`longpoll`** wait on tip/mempool wake (`rpc/getblocktemplate.go`, `gbt_longpoll.go`)
- [x] **MVP:** mempool **rolling min fee** after eviction uses configured **`incrementalrelayfee`** (`mempool.Pool.SetIncrementalRelayFeePerKB`, `node/run.go`)
- [x] **MVP:** obsolete block version uses **base version** (`BlockBaseVersion`, Core `GetBaseVersion`)
- [x] **MVP:** **version-bit / unexpected-version warnings** on `getblockchaininfo` / `getnetworkinfo` (`consensus/chain_warnings.go`)
- [x] **MVP:** **`alertnotify`** shell hook on chain-warning changes (`node/alertnotify.go`, `dogecoinconf.json` / `-alertnotify`; skipped during forward block IBD)
- [x] **MVP:** Web UI **chain rule-change banner** on Overview (`chain_warnings` from `getblockchaininfo`; Core Qt alerts analogue)

---

## Phase 5 - Persistence (native only)

DogeGo does **not** read or write Dogecoin Core `blocks/` + `chainstate/` LevelDB. All chain data uses the native layout under `<datadir>/<network>/`: `headers/seg/` (or legacy `headers.bin`), `rawblocks/*.bin`, `indexes/tx/`, optional `utxo.cache`, wallet scan cache. Wire/RPC semantics stay Core-compatible where it matters (`-assumevalid`, `hash_serialized`, `fee_estimates.dat`); on-disk layout does not.

- [x] **MVP:** Sequential raw header journal (`store/journal.go`) - **default: segment files** under `headers/seg/` via `OpenHeaderChain`; legacy monolith still supported
- [x] **MVP:** Native raw block store (`store/rawblock.go`) + contiguous body height for IBD/RPC
- [x] **MVP:** Native tx index (`store/txindex.go`) for `getrawtransaction` / explorer - v2 entries embed serialized tx (36-byte legacy entries still read); **`reindextx`** / **`upgradetxindex`** RPC for operator maintenance
- [x] **MVP:** In-memory UTXO cache at tip + `gettxoutsetinfo` / wallet scans over `rawblocks/` (no Core chainstate); **`ApplyBlockRaw`** on ConnectBlock path and IBD rebuild (streaming txs, no full-block slice)
- [x] **MVP:** Wallet UTXO scan cache (`wallet_utxo_scan.cache.json`)
- [x] **MVP:** optional **Pebble analytics side-DB** (`dogego_analytics.db`; `dogego indexer` init/status/scan/reindex-tx/verify-bodies; embedded sidecar during node/spv)
- [x] **Perf - headers:** parallel header assist peers during startup probe (`node/header_assist_sync.go`); relay `getheaders` top-up during steady state (`headers_topup_multi.go`)
- [x] **Perf - blocks:** scaled `getdata` batch size with sync lanes (`EffectiveProgressiveBatchSize`); block-assist + `-assumevalid` script skip during IBD
- [x] **Perf - RPC:** LRU raw-block read cache for hot `getblock` (`store/rawblock_readcache.go`); `getblock` verbosity **0**/**1**/**2** avoid retaining full `ParsedBlock` (v2 uses `ForEachBlockTx` + `RPCTxidsFromPayload`); `getblockstats` streams txs via `ForEachBlockTx` + **`ComputeBlockFeeStatsRaw`** / **`BlockUtxoSizeIncreaseRaw`** (no `ParseBlock`);
- [x] **Perf - consensus:** `CheckBlockPayload` on raw bytes - merkle/header, **auxpow**, duplicate txid/spend, **block weight** + **base size**, BIP34 coinbase height, per-tx `CheckTransaction` (`BlockWeightRaw`, `BlockHeaderAuxFromPayload`); used by `StoreValidatedBlock`, `submitblock`, `validate_stored_blocks`, GBT proposal;
- [x] **Perf - connect:** `ConnectBlockRaw` + `RecordBlockConnectedRaw` / `BlockTxFeeSamplesRaw` / `CheckBlockSigOpCostRaw` - IBD frontier and block store connect use raw bytes only (`tryConnectBlockPayloadRaw`, `ApplyBlockRaw`; no `ParseBlock` fallback on connect path); `getrawtransaction` blockhash filter + explorer address scan use `FindTxByRPCID` / `ForEachBlockTx`; inbound `StoreValidatedBlock` uses `ValidateBlockPayload` before connect; `getrawtransaction` / P2P `tx` serve from tx-index v2 raw when present, else `wire.ReadTxAtIndex`; `ChainPrevOutView` uses `ReadTxAtIndex` on legacy index path; summary API uses cached contiguous height (`BlockStoreCtx`)
- [x] **Perf - mempool:** `RemoveForBlockRaw` prunes from serialized block without `ParseBlock`; `CollectMempoolConfirmedSamplesRaw` on block store sideband (no parse for fee samples)
- [x] **Perf - filters / wallet / RPC:** BIP158 basic filter build (`blockfilter_build`) and wallet `ScanBlocksRange` use `ForEachBlockTx`; `gettxoutproof` streams tx hashes; UTXO spend check uses `ReadTxAtIndex`
- [x] **Perf - ops:** `verifychain` / `getblockstats` / block sideband use raw-block paths only (no production `ParseBlock` on connect, stats, or orphan promote)
- [x] **Perf - tx index:** background upgrade of legacy 36-byte `indexes/tx` entries to v2 (`store/txindex_upgrade.go`; node polls every 15m); `getindexinfo` reports `dogego_legacy_files` / `dogego_v2_files`; **`upgradetxindex`** / **`reindextx`** RPC; **`LoadIndexedTx`** / **`LoadIndexedTxVout`** fast path for `gettxout`, wallet/PSBT prevouts, filter build, **`scanblocks`**; **`wire.ForEachBlockTx`** / **`BlockTxCount`** for index, spend scan, UTXO rebuild, **`getchaintxstats`** window
- [x] **Core parity:** `getindexinfo.coinstatsindex` exposes `hash_serialized` when UTXO cache matches chainActive (no separate coinstats LevelDB index)
- [x] **Core parity:** difficulty retarget vectors from Core `pow_tests.cpp` / `dogecoin_tests.cpp` (pre-Digishield + Digishield + modulation/rounding; `consensus/difficulty_test.go`; shared `pow.CompactFromBigInt`)
- [x] **Core parity:** `getNextWorkRequired` uses consensus params at **parent height** (`pindexLast`), not child - matches Core `GetNextWorkRequired` at era boundaries (`headers_validate.go`, `validate_stored.go`)
- [x] **Core parity:** `getrpcinfo` exposes **`authentication_failures`**, **`method`** map, and **`dogego_rpc_tls`** when native RPC TLS is enabled
- [x] **Perf - wallet:** in-process `sendtoaddress` via `WalletSendBridge` + web `/api/wallet/send` (no Core wallet RPC)
- [x] **Perf - wallet (solo miner):** `getwalletinfo` single-pass UTXO balance scan + `spendable_utxo_count`; `walletUniqueTxCount` without full `walletCollectTransactions`; compact tx-index **immature coinbase** vout-0 heuristic (`tx_index_embed_tx: false`); **`walletCollectTransactionsUI`** light path + row cache for RPC wallet list (`rpc/wallet_ui_list.go`); web `/api/wallet` + `/api/wallet/txs` + utxos prefer live UTXO cache (`ui/wallet_fast.go`, `rpc/wallet_meta.go`, `rpc/wallet_misc.go`, `rpc/wallet_ui_list.go`); **`fundrawtransaction`** O(n log n) input sort + estimated tx size for selection (`rpc/wallet_fund_sort.go`, `fundrawtransaction.go`)
- [x] **Perf - mining:** GBT block **proposal** uses `CheckBlockPayload` + `ConnectBlockRaw` (no `ParseBlock`; `rpc/getblocktemplate_proposal.go`); **`submitblock`** / inbound **`StoreValidatedBlock`** same raw path (`ExtendHeadersFromPayload`, `tryConnectBlockPayloadRaw`); **`verifychain`** body walk uses **`ValidateStoredBlockBodies`** (`CheckBlockPayload` + `ConnectBlockRaw`); block sideband orphan promote via **`PromoteOrphansForBlockRaw`**
- [x] **Web UI:** native storage line on Overview (contiguous bodies, tx-index v2/legacy counts); PIN status in Settings; setup review shows PIN choice; no PIN prompt when `pin_enabled` is false (status API + overlay CSS)

---

## Phase 6 - Mempool & relay policy

- [x] **MVP:** In-memory pool (`mempool/pool.go`); **default full-node policy:** reject coinbase + unverified spends (`consensus.AcceptMempoolTx` on `sendrawtransaction` and inbound P2P `tx`); opt out with `-allowunverifiedmempool` / `allow_unverified_mempool` in `dogecoinconf.json` (testing only)
- [x] **MVP:** P2P orphan pool + promotion (`mempool/orphan.go`, `consensus/mempool_orphan.go`)
- [x] **MVP:** min relay feerate, package limits, RBF, ancestor-scored eviction (partial vs Core)
- [x] **Core parity:** BIP125 **PaysForRBF** conflict-set fees use **descendant** package (`ConflictPackageFeeSize` → `DescendantFeesKoinu`/`DescendantSize`; underpay child reject + ancestor-not-charged tests)
- [x] **MVP:** multi-peer **feefilter** aggregate (max across connected peers for mempool hints; per-peer filter for tx relay to relays)
- [x] **MVP:** orphan pool **eviction when full** (drop one orphan before add; Core-style cap; configurable via `maxorphantx`) (`mempool/orphan.go`)
- [x] **MVP:** **min standard tx size** (82-byte non-witness minimum, Core `MIN_STANDARD_TX_NONWITNESS_SIZE`; `consensus/standard.go`)
- [x] **MVP:** **`-maxmempool`** byte cap (default 300 MB) + ancestor-fee eviction when over cap (`mempool/pool.go`, `AddWithEviction`)
- [x] **MVP:** **`-mempoolexpiry`** (default 24h) periodic prune (`mempool/prune.go`, `dogecoinconf.json` `mempoolexpiry`)
- [x] **MVP:** orphan **MAX_STANDARD_TX_WEIGHT** admission cap + **20 min expiry** sweep (Core `ORPHAN_TX_EXPIRE_TIME`)
- [x] **MVP:** mempool **percentile feerate hint** for `estimatesmartfee` / `estimatefee` when prevouts resolve (`consensus/fee_estimate.go`, `MempoolFeeEstimate` in RPC paths)
- [x] **MVP:** orphan **EraseOrphansFor** on peer disconnect (`mempool/orphan.go` `RemoveByPeer`, `node/peermgr.go`)
- [x] **MVP:** **confirmed-block feerate history** on `ConnectBlock` for `estimatesmartfee` (`consensus/fee_history.go`, `ConfirmedFeeEstimate` RPC path)
- [x] **MVP:** **`fee_history.json`** persist/load under chain datadir (Core `fee_estimates.dat` analogue)
- [x] **MVP:** **depth-scoped fee buckets** (1-144 block targets) on `estimatesmartfee` / `getmempoolinfo` (`dogego_fee_buckets`, `FeeHistory.BucketEstimatesDOGE`)
- [x] **MVP:** confirmed-fee **horizon max** (1..nblocks depth targets) for longer `estimatesmartfee` horizons (`FeeHistory.EstimatePerKBHorizonMax`)
- [x] **MVP:** confirmed-fee **decay weighting** (48-block halflife) + live **`mempool_sequence`** on `getmempoolinfo` (`FeeHistory.EstimatePerKBDecay`, `mempool.Pool.ChangeSequence`)
- [x] **MVP:** `estimatesmartfee` **conservative** / **economical** modes + bucket-target `blocks` field (`rpc/estimatefee.go`, `FeeHistory.EstimatePerKBConservative`)
- [x] **MVP:** per-target **fee bucket market stats** persisted in `fee_history.json` (`dogego_fee_bucket_market` on `getmempoolinfo` / `estimatesmartfee`)
- [x] **MVP:** **mempool-confirmed feerate samples** on block store (before `RemoveForBlock`; `fee_history.json` `mempool_confirmed`; feeds `estimatesmartfee` conservative path)
- [x] **MVP:** **blocks-to-confirm fee buckets** for mempool-confirmed txs (`addedAtHeight`, `mempool_confirm_buckets` on `getmempoolinfo` / `estimatesmartfee`)
- [x] **MVP:** persist **`fee_history.json`** after mempool-confirmed block samples and on **`ConnectBlock`** (`node/run.go` sideband, `BlockStoreCtx.FeeHistoryPath`)
- [x] **MVP:** `estimatesmartfee` **economical** confirmed path uses mempool-confirm percentiles + block median (`EstimateMempoolConfirmedPerKBEconomical`)
- [x] **MVP:** **mempool admission feerate tracking** for fee estimator (`FeeHistory.TrackMempoolAdmission`, P2P/RPC accept + untrack on pool remove; `dogego_mempool_fee_pending_tracks` on `getmempoolinfo`)
- [x] **MVP:** **left-mempool-without-confirm** feerate samples on eviction/expiry (`RecordMempoolLeftWithoutConfirm`; conservative `estimatesmartfee`; `dogego_mempool_left_without_confirm` on `getmempoolinfo`)
- [x] **MVP:** **left-mempool fee buckets** by blocks-in-mempool horizon (`dogego_mempool_left_buckets` on `getmempoolinfo` / `estimatesmartfee`; `FeeHistory.MempoolLeftBucketStats`)
- [x] **MVP:** `getrawmempool` verbose / `getmempoolentry` **`modifiedfee`** reflects `prioritisetransaction` fee deltas (`mempool.Pool.FeeDeltaKoinu`)
- [x] **MVP:** `prioritisetransaction` propagates **`fee_delta`** to ancestor/descendant mining scores (Core `CTxMemPool::PrioritiseTransaction`; `MiningAncestorFeesKoinu`, GBT + eviction)
- [x] **Core parity:** `prioritisetransaction` **latent mapDeltas** - fee_delta accepted before tx is in mempool; applied on admit (`mempool/priority_delta.go`, `TestLatentFeeDeltaBeforeMempoolAdmit`); zero delta removes entry (Core `mapDeltas` erase)
- [x] **Core parity:** persist **`fee_deltas`** in `dogego_mempool.json` v2 on `savemempool` / shutdown / `loadmempool` restore (Core `mapDeltas` in `mempool.dat`; `mempool/persist.go`, `TestSaveLoadPersistedFeeDeltasRoundTrip`)
- [x] **MVP:** **`testmempoolaccept`** Core-shaped **`fees`** object when prevouts resolve (incl. on reject / duplicate-in-mempool; `modified` includes fee delta)
- [x] **MVP:** persist **`fee_history.json`** when tracked txs **leave mempool unconfirmed** (eviction/expiry; `RecordMempoolLeftWithoutConfirm` → save)
- [x] **MVP:** `estimatesmartfee` **economical** mode uses **minimum** of market sources (not max); `FeeHistory.EstimatePerKBEconomical` + low mempool percentile (`EstimateMempoolFeePerKBEconomical`)
- [x] **MVP:** exponential **feerate confirm buckets** (`TxConfirmStats`, Core `FEE_SPACING` 1.05 / `DEFAULT_DECAY`); mempool-confirm samples feed `EstimatePerKBFromConfirmStats`; persisted in `fee_history.json`; `dogego_fee_confirm_stats` on `getmempoolinfo` / `estimatesmartfee`
- [x] **MVP:** **unconf feerate bucket** tracking on mempool admission (`TxConfirmStats.TrackMempoolTx` / `RemoveMempoolTx`; `NotifyBlockHeight` on block store; feeds conservative confirm-stats estimate)
- [x] **MVP:** `estimatesmartfee` **target walk** for confirm-stats buckets (Core `estimateSmartFee`; `EstimateConfirmStatsSmart`; `blocks` field reflects found target)
- [x] **MVP:** Core **`fee_estimates.dat`** binary read/write for `TxConfirmStats` + `nBestSeenHeight` (`consensus/fee_estimates_dat.go`; load overlays JSON; save with `fee_history.json`)
- [x] **MVP:** fee estimator **mempool track rehydrate** after `dogego_mempool.json` restore (`FeeHistory.RehydrateFromPool`; Core `mapMemPoolTxs` analogue)
- [x] **MVP:** read legacy Core **`fee_estimates.dat`** with trailing priority `TxConfirmStats` (pre-139900) skipped
- [x] **MVP:** persist **`pending_mempool`** fee tracks in `fee_history.json` + restore `TxConfirmStats` unconf at tip (`ApplyLoadedPendingTracks`; Core `mapMemPoolTxs` analogue)
- [x] **MVP:** persist **`TxConfirmStats` unconf ring** + `mempool_tracks` in `fee_history.json`; `CatchUpBlockHeights` on restart
- [x] **MVP:** deprecated **`estimatefee`** returns **-1** for **nblocks=1**; **`estimatesmartfee`** walks target **2** (Core `estimateSmartFee` confTarget bump)
- [x] **MVP:** **`getblockchaininfo.warnings`** / **`getnetworkinfo.warnings`** = chain warnings only; DogeGo status in **`dogego_status_note`** (Core-shaped `warnings` field)
- [x] **MVP:** **`getinfo.errors`** includes chain warnings (Core deprecated aggregate)
- [x] **Core parity:** **`getblockchaininfo`** **`blocks`** vs **`headers`** - `blocks` = **chainActive** (UTXO/connect tip), `headers` = header journal tip; `dogego_contiguous_raw_height` = stored bodies; `bestblockhash` / difficulty / mediantime / softforks at **blocks** height
- [x] **Core parity:** **`getmininginfo.blocks`** uses same chainActive height as `getblockchaininfo.blocks`
- [x] **Core parity:** **`getblockcount`** / **`getbestblockhash`** / **`getdifficulty`** / wallet / mining / scan RPCs use **chainActive** (`paths.Utxo` when wired), not header tip or orphan bodies ahead of `ConnectBlock`
- [x] **MVP:** **`submitblock`** / block extend use **`ExtendHeadersFromParentHeight`** (build on chainActive tip, not header journal tip during body lag)
- [x] **MVP:** **`getchaintips`** reports **active** chain tip + **`headers-only`** tip when headers run ahead of stored bodies; **`gettxoutsetinfo`** / **`getchaintxstats`** default to chainActive height
- [x] **Core parity:** **`getblockheader`** (no params) / **`confirmations`** on **`getblock`** / **`getblockheader`** use **chainActive** tip; **`getindexinfo.txindex`** **`best_block_height`** + **`synced`** vs chainActive; **`dumptxoutset`** / **`loadtxoutset`** at chainActive; **`waitfor*`** RPCs poll **chainActive**; **`NotifyRPCTip`** on **ConnectBlock** / contiguous advance; **`gettxout`** / **`getrawtransaction`** confirmations + **`in_active_chain`**; **`getnetworkhashps`** / **`getdeploymentinfo`** default tip at chainActive; fee **`HeaderTipHeight`** = chainActive during IBD (`node/chain_active.go`, `run.go`)
- [x] **MVP:** built-in **wallet** RPCs (`listunspent`, balances, `listtransactions`, …) use **chainActive** for confirmations and minconf; **`verifychain`** default window at chainActive tip; **`pruneblockchain`** capped to chainActive height
- [x] **MVP:** **`listsinceblock`** **`lastblock`** at **chainActive**; wallet path filters confirmed receives to blocks after `blockhash`; **`rescan`** / **`importprivkey`** height capped to chainActive; web **chainstats** / address scan use chainActive tip during IBD
- [x] **MVP:** wallet **`listtransactions`** / **`gettransaction`** / **`listunspent`** expose **`blockhash`** / **`blockheight`** / header **`time`**; **`listunspent`** honors **`maxconf`**
- [x] **MVP:** **`listreceivedbyaddress`** / **`listreceivedbylabel`** / **`listreceivedbyaccount`** include Core-shaped **`txids`**; **`include_watchonly`** honored on listreceived; **`invalidateblock`** notifies **`waitfor*`** at chainActive tip
- [x] **MVP:** wallet **`listtransactions`** / **`gettransaction`** - Core **`bip125-replaceable`** (`yes`/`no`/`unknown` from mempool), **`walletconflicts`** (bumpfee replacements in `wallet.json` + mempool double-spends among wallet txs), **`trusted`** when confirmed; web **`tx_kind`** / **`pq_tag`** enrichment for History UI (`walletEnrichTxKind`, `walletPQTagFromTxHex`)
- [x] **MVP:** **`gettransaction`** **`hex`** from mempool / txindex / raw block scan; mempool **`fee`** on sends; **`liststucktransactions`** **`include_watchonly`** + **`verbose`** (`hex`/`fee`)
- [x] **MVP:** AuxPoW rejects **chain/coinbase merkle branches >30**, **multiple merged-mining headers**, **merged-mining header not immediately before chain root**, and legacy coinbase root **after byte 20** without merged-mining header (Core `CAuxPow::check`)
- [x] **MVP:** `ConnectBlock` **`RecordBlockConnected`** - untracked block txs → `RecordConfirm(1)` + `FlushBlock`; skips mempool-confirmed txids (Core `processBlock` / `UpdateMovingAverages`)
- [x] **MVP:** persist **`TxConfirmStats` cur-block batch** (`cur_conf` / `cur_tx_ct` / `cur_val` in `fee_history.json`; Core in-flight block before `FlushBlock`)
- [x] **MVP:** deprecated **priority fee RPCs** match Core stubs - `estimatepriority` / estimator always **-1**; `estimatesmartpriority` returns **`INF_PRIORITY`** when min relay is enforced (`consensus.InfPriority`; Core `CBlockPolicyEstimator::estimateSmartPriority`)
- [x] **MVP:** surface inbound P2P `feefilter` in `getnetworkinfo` / `getmempoolinfo` (`relayfee` / `minrelaytxfee` = effective min relay via `minRelayFeeFromPaths`: max peer feefilter, mempool rolling floor, chain default; `dogego_mempool_rolling_minfee`, `dogego_max_orphan_tx`; koinu/kB → DOGE/kB display)
- [x] **MVP:** `getnetworkinfo` / `getinfo` **`timeoffset`** = median of connected peers' version **nTime** offsets (`PeerMgr.MedianTimeOffset`; per-peer offset on `getpeerinfo`); **`localaddresses`** from inbound listen bind + connected session local endpoints (`PeerMgr.LocalAddressRows`); **`networks[]`** ipv4/ipv6/onion rows (onion unreachable; no Tor proxy in DogeGo)
- [x] **MVP:** per-peer **`synced_headers`** / **`synced_blocks`** on `getpeerinfo` from peer version start height + headers/block delivery (`NotePeerHeaders`, `NotePeerBlockAt`)
- [x] **MVP:** **`setban` add** and misbehavior auto-ban **disconnect matching relay peers** (`BanDisconnect`, `DisconnectBanned`; primary sync peer unchanged)
- [x] **MVP:** configurable **`incrementalrelayfee`** (`dogecoinconf.json` koinu/kB; `consensus/incremental_relay.go`; BIP125 / `bumpfee` / `getmempoolinfo` / `getnetworkinfo`)
- [x] **MVP:** configurable **`minrelaytxfee`** + Core startup bump when only incremental is set (`consensus/min_relay.go`, `ApplyNodeRelayFees`; admission / dust / RPC min relay floor)
- [x] **MVP:** **`getblocktemplate` proposal** mode - Core BIP22 (`duplicate`, `inconclusive-not-best-prevblk`, `TestBlockValidity` via `ConnectBlock`; no require-all-txs-in-mempool)
- [x] **MVP:** `getconnectioncount` returns live **P2P session** count (primary + relays; excludes block-assist workers; Core `CONNECTIONS_ALL` analogue)
- [x] **MVP:** **`ping`** RPC queues immediate outbound P2P **ping** on all connected peers (`PeerMgr.PingAll`; results in `getpeerinfo` **pingtime** / **pingwait**)
- [x] **MVP:** **`getnetworkinfo.connections`** = P2P session count (not block-assist workers); **`dogego_block_assist_connections`** / **`dogego_connections_with_assist`** when IBD assist active
- [x] **MVP:** **`setnetworkactive` false** disconnects relay peers (`DisconnectAllRelays`; primary unchanged)
- [x] **MVP:** **`listbanned` / `setban`** permanent bans (`banned_until` 0 never expires; Core absolute-time analogue)
- [x] **MVP:** web **Settings** + setup wizard expose **mempool relay policy** (`maxmempool`, `mempoolexpiry`, `minrelaytxfee`, `incrementalrelayfee`, `mempoolfullrbf`; `ui/static/index.html`, `setup.html`)
- [x] **MVP:** configurable **package limits** (`limitancestorcount`, `limitdescendantcount`, `limitancestorsize`, `limitdescendantsize`) + **standardness** (`acceptdatacarrier`, `datacarriersize`, `permitbaremultisig`); `getmempoolinfo` **`dogego_package_policy`** / **`dogego_standard_policy`**; web dashboard
- [x] **MVP:** **`savemempool`** / **`loadmempool`** - JSON dump **`dogego_mempool.json`** under chain dir (not Core `mempool.dat`); auto-restore on startup + save on shutdown / `stop` (`mempool/persist.go`, `rpc/mempool_persist.go`, `node/run.go`)
- [x] **MVP:** **`persistmempool`** config (default on; disable auto load/save; manual `savemempool`/`loadmempool` still work)
- [x] **MVP:** fee estimator uses **per-target block median buckets** (`FeeHistory.EstimatePerKBFromBucketMedians`; feeds `estimatesmartfee` conservative path)
- [x] **MVP:** `getmempoolinfo` **`feerate_percentiles`** (vsize-weighted 10/25/50/75/90 when prevouts resolve); **`permitbaremultisig`** / **`maxdatacarriersize`** (Bitcoin 30-shaped standardness fields)
- [x] **MVP:** **`setmempoolpaused`** RPC toggles mempool admission (`mempool.Pool.SetPaused`)
- [x] **MVP:** **`getblockstats`** fee fields when prevouts resolve - `totalfee`, `min/max/medianfee`, `avg/min/maxfeerate`, `feerate_percentiles` (koinu/byte; UTXO cache at parent height + tx index)
- [x] **MVP:** **`getblockstats`** **`dustouts`** + approximate **`utxo_size_inc`** when prevouts resolve
- [x] **MVP:** **`getchaintxstats`** **`window_tx_count`** from stored **rawblocks** in range; **`window_final_block_hash`**

---

## Phase 7 - RPC & operations

- [x] **MVP:** HTTP JSON-RPC listener, `getblockchaininfo` subset (`difficulty`, `mediantime` MTP-style from headers, `softforks` / `bip9_softforks` from consensus heights; `rpc/dispatch.go`, `rpc/softforks.go`)
- [x] **MVP:** `getblock` verbosity **0 / 1 / 2** (hex, txid list, full tx JSON from `wire.Tx` + `Serialize` / `WTxHash`) when block is in `rawblocks/`
- [x] **MVP:** **`getblockfilter`** BIP158 **basic** Golomb-Rice filter from **`rawblocks/`** + tx-index prevouts (`consensus/gcs_filter.go`, `rpc/getblockfilter.go`; filter header chain vs parent block)
- [x] **MVP:** **persisted block filters** under **`filters/basic/`** (indexed on raw block store; repair backfill; **`getindexinfo`** **`basic block filter`**)
- [x] **MVP:** **`getblockfilterheader`**; **`getblockchaininfo.filters`** (`active` + `height`); BIP157 P2P **`getcfilters`/`cfilter`**, **`getcfheaders`/`cfheaders`**, **`getcfcheckpt`/`cfcheckpt`** when tx index + filter index enabled
- [x] **MVP:** BIP157 **`NODE_COMPACT_FILTERS`** (`1<<6`) on P2P **`version`** / **`addr`** when tx index + filter index enabled (`chain.EffectiveP2PServices`, `node/handshake.go`, `PeerMgr.SetLocalServices`)
- [x] **MVP:** BIP158 basic filter **byte match** vs full Bitcoin Core `src/test/data/blockfilters.json` (10 vectors incl. **49291**, **180480**, **926485** duplicate pushdata, **1263442** witness-shaped block bytes; `consensus/blockfilter_core_test.go`, `consensus/siphash.go` aligned with Core `CSipHasher`)
- [x] **MVP:** **`reindexblockfilters`** RPC - force-rebuild **`filters/basic/`** from **`rawblocks/`** + tx index (`store.RebuildBlockFiltersFromRaw`, `rpc/reindexblockfilters.go`)
- [x] **MVP:** **`scanblocks`** - BIP158 **basic** filter scan for **`addr`/`pkh`/`sh`/`multi`/`raw`** descriptors (`start`/`abort`/`status`; optional **`filter_false_positives`** block verify via raw blocks + tx index)
- [x] **MVP:** `sendrawtransaction` (hex → decode → **consensus mempool admission** unless `-allowunverifiedmempool`; optional **P2P `inv`(MSG_TX)** relay per peer feefilter; peers fetch via `getdata`) (`rpc/sendrawtransaction.go`, `node/relay_tx.go`)
- [x] **MVP:** **`testmempoolaccept`** dry-run admission + optional **maxfeerate** (DOGE/kB); does not broadcast (`rpc/testmempoolaccept.go`)
- [x] **MVP:** **`submitpackage`** - topologically sorted parent→child package admission + relay (`package_msg`, `tx-results` keyed by wtxid; default **maxfeerate** 0.10 DOGE/kB; **maxburnamount** for OP_RETURN outputs)
- [x] **Core parity:** **`submitpackage` CPFP** - package-unit min relay (`CheckMinRelayFeePackageTxs` + `SkipMinRelayFee`); **`fees.effective-includes`**; **`replaced-transactions`** on BIP125 replace
- [x] **Core parity:** **`createmultisig`** rejects **duplicate keys** in the pubkey list (Core `addmultisigaddress` / `createmultisig`; `buildMultisigRedeemScript`; same rule for **`getdescriptorinfo`** / **`importdescriptors`** on `sh(cltv|csv)multi(...)`)
- [x] **MVP:** **`testmempoolaccept`** Core reject strings - **`txn-already-known`** (confirmed tx), **`mempool full`**, **`Missing inputs`** (orphan path via `acceptMempoolTxRPC`)
- [x] **MVP:** Core-shaped **`reject-reason`** strings from mempool admission (`consensus.MempoolRejectReason`; `testmempoolaccept`, `sendrawtransaction`, P2P BIP61)
- [x] **MVP:** **`sendrawtransaction`** Core paths - **`-25` Missing inputs**, **`-27` already in chain**, re-broadcast when already in mempool; optional orphan queue via `DataPaths.OrphanPool` (still returns Missing inputs to caller)
- [x] **MVP:** `getrawmempool` (optional verbose; fee fields when prevouts resolve) and `getmempoolinfo` (`rpc/mempool_rpc.go`, `mempool/pool.go`)
- [x] **MVP:** `getmempoolinfo` Core fee fields - **`minrelaytxfee`** (configured) vs **`mempoolminfee`** (effective incl. rolling/feefilter); **`mempoolexpiry`** hours; **`paused`** when mempool relay is paused
- [x] **MVP:** `estimatesmartfee` **`errors`** entries use Core-shaped `{type, message}` objects; **`getmininginfo.errors`** includes chain warnings
- [x] **MVP:** **`savemempool`** / **`loadmempool`** RPC (`dogego_mempool.json`; reload applies current relay policy)
- [x] **MVP:** **`importmempool`** - import **`dogego_mempool.json`** (or compatible dump) from a filepath (`rpc/importmempool.go`; not Core `mempool.dat`)
- [x] **MVP:** **`psbtbumpfee`** - wallet fee bump as signed PSBT (`origfee`/`fee`; no broadcast; shares `buildWalletBumpFeeTx` with **`bumpfee`**)
- [x] **MVP:** **`simulaterawtransaction`** - wallet balance delta (DOGE) for raw hex txs using UTXO cache + mempool prevouts
- [x] **MVP:** **`setmempoolpaused`** JSON-RPC (operator pause/resume; matches `getmempoolinfo.paused` and web `/api/services` mempool actions)
- [x] **MVP:** **`getchaintxstats`** window tx count from **rawblocks** + **`window_final_block_hash`**
- [x] **MVP:** `getrawmempool` verbose **`height`** = chain tip height at mempool admission (`addedAtHeight`, `SetTipHeightFn`)
- [x] **MVP:** `getmempoolentry` / verbose mempool **`startingpriority`** / **`currentpriority`** = 0 (Core deprecated fields)
- [x] **MVP:** **`getmemoryinfo`** exposes Go **`runtime.MemStats`** heap fields (Core `locked` shape; `dogego_heap_*` metadata)
- [x] **MVP:** `getrawmempool` **verbose=true** returns a **JSON object keyed by txid** (Core `mempoolToJSON`; not an array)
- [x] **MVP:** `getrawtransaction` / `decoderawtransaction`; confirmed txs from `indexes/tx` + `rawblocks/`, **mempool fallback** (Core-style “chain then mempool”); web `GET /api/explorer/tx` includes `source`
- [x] **MVP:** `gettxout` (Core-shaped fields; uses in-memory UTXO cache when synced to tip, else scans **rawblocks** from coin height through tip plus mempool when `include_mempool`)
- [x] **MVP:** In-memory UTXO cache at connected tip (`store/utxocache.go`); `gettxoutsetinfo` when cache matches **chainActive** tip (optional **`SyncUtxo`** refresh before stats); optional `utxo.cache` snapshot (not Core LevelDB chainstate)
- [x] **MVP:** `getnetworkhashps` uses Digishield-aware retarget interval per height (`consensus.DogeConsensus.DifficultyAdjustmentBlocks`)
- [x] **MVP:** Mempool on-chain double-spend checks via UTXO cache when synced (`consensus/chain_spend_utxo.go`)
- [x] **MVP:** `gettxoutproof` / `verifytxoutproof` (Core **CMerkleBlock** / partial Merkle encoding; child **80-byte** header in proof - auxpow payload excluded, matches `headers.bin` / `verifytxoutproof`)
- [x] **MVP:** `decodescript` (Core-shaped `asm` / `type` / `addresses` / `p2sh` for standard templates; partial opcode disassembly for other scripts) and **`createrawtransaction`** (unsigned legacy tx: P2PKH + P2SH outputs for chain version bytes, `data` OP_RETURN ≤80 bytes, optional `locktime`)
- [x] **MVP:** `validateaddress` for **P2PKH** on the RPC chain (`main` / `testnet` / `test` alias); `chain.Base58CheckDecode` (`chain/address.go`, `rpc/validateaddress.go`)
- [x] **MVP:** `estimatesmartfee` / `estimatefee` (effective min relay: max peer **feefilter** + mempool rolling floor; `estimatefee` returns **-1** if none) (`rpc/estimatefee.go`, `minRelayFeeFromPaths`)
- [x] **MVP:** Core-style **`.cookie`** auth when `rpc_cookie` is set (`rpc/cookie.go`, `node/run.go`); HTTP Basic via `rpc_user` / `rpc_password` in `dogecoinconf.json` (`rpc/auth.go`, `node/run.go`)
- [x] **MVP:** JSON-RPC **batch** requests (array of calls → array of responses; `rpc/server.go`)
- [x] **MVP:** `rpcwhitelist` method allowlist (`rpc/rpcwhitelist.go`, `rpc/dispatch.go`)
- [x] **MVP:** per-IP JSON-RPC **`rpclimit`** + failed-auth **`rpcauthmaxfail`** (default 30/min when auth on; `-1` disables auth fail cap) (`rpc/rpclimit.go`)
- [x] **MVP:** Native TLS for JSON-RPC + web UI via `rpc_tls_cert`/`rpc_tls_key` and `webui_tls_cert`/`webui_tls_key` in `dogecoinconf.json` (`httptls/listen.go`; TLS 1.2+)

---

## Phase 8 - Web dashboard & operator UX

- [x] **MVP:** loopback web UI (`ui/static/index.html`), hash routing, explorer APIs, analytics tab, **Guide** section (how DogeGo relates to Core, analytics, PQC roadmap).
- [x] **MVP:** web explorer **P2SH address** local window scan (`ScanAddressInRawWindow` matches P2PKH + P2SH vouts via `ScriptPubKeyAddress` / `ForEachBlockTx`)
- [x] **MVP:** **Docs** tab + `GET /api/docs` - integrator/operator documentation hub (`ui/docs_index.go`; mirrors `docs/DOCUMENTATION.md`, `INTEGRATION.md`, …).
- [x] **MVP:** **Mempool** tab + Overview show **minrelaytxfee** / **incrementalrelayfee** from live pool + config (`/api/mempool`, `/api/summary` `relay_policy`).
- [x] **MVP:** **Settings** + first-run **setup** edit Core-style relay/mempool fields in `dogecoinconf.json` (restart required).
- [x] **MVP:** Settings/setup **package limits** and **standardness** flags; Mempool tab + Features live strip show effective policy (`/api/summary` `relay_policy`, `/api/mempool`).
- [x] **MVP:** browser-only **display preferences** (poll interval, optional raw `/api/summary`, P2P JSON visibility, compact layout) via `localStorage` - no substitute for server-side security.
- [x] **MVP:** **modern dashboard content** - refreshed metric cards with sparkline charts (Chart.js), gradient main area; top bar + sidebar layout preserved.
- [x] **MVP:** **click-to-open help** popovers on fields (`?` buttons); **simple dashboard mode** (default on first visit - Overview + Wallet + Settings only) + per-item **sidebar visibility** (Settings → Interface).
- [x] **MVP:** **first-run setup wizard (2025 refresh)** - profile cards (testnet / mainnet / mainnet+wallet / SPV+wallet), 5-step flow, **`POST /api/setup/preflight`** (port bind, firewall rules, CGNAT hints + fix commands), advanced options collapsed unless toggled (`ui/static/setup.html`, `ui/setup_preflight.go`).
- [x] **Web UI polish:** mobile bottom-nav parity with simple mode (Send + BlockStep on bottom bar; advanced tabs stay hidden in simple mode) - **S**
- [x] **MVP:** **6-digit dashboard PIN** - bcrypt hash in `web_security.json`, 3 failures → 1h lock, httpOnly session cookie; gates `/api/wallet*` (setup wizard + Settings → Security); topbar **Lock** shows PIN overlay immediately; dashboard polls `/api/security/status` during refresh.
- [x] **MVP:** **Lock** / **Lock now** (`topbar-lock`, `st-sec-lock`) - `userWantsLocked` survives dashboard refresh race; overlay stays until PIN unlock (`ui/static/security.js`)
- [x] **MVP:** web dashboard **live feed resilience** - rate-limited `rawblocks/` count reconcile, P2P snapshot timeout + 750ms cache, bootstrap `/api/live` when summary build blocks during IBD (`ui/live.go`, `ui/raw_count_reconcile.go`, `ui/p2p_snapshot_timeout.go`, `node/run.go`)
- [x] **WebAuthn** server verification (platform biometrics bound to credential via `/api/security/webauthn/*`) - **M**
- [x] **Richer charts / filters on analytics; export CSV** - timeline window (1h/6h/24h/all) + `GET /api/analytics/metrics.csv` - **S**
- [x] **Web UI polish:** **Authenticated remote dashboard** - `webui_remote_auth` + dashboard PIN session for non-loopback reads when `webui` binds beyond loopback; remote `/api/security/unlock` - **S** (`ui/remote_access.go`, Settings Interface)
- [x] **MVP:** In-dashboard **markdown viewer** for full `docs/*.md` (render repo files; Docs tab + `GET /api/docs/md` + marked.js)
- [x] **MVP:** **Core parity web UI** - live probe bundle (`/api/core-probes`, `/api/core-operator-cert`), operator cert matrix (live + script-only soak gates), Overview/sync-dock status, **UTXO body replay** progress on summary/capabilities live strip, Console probe runner, Settings Core RPC test (`POST /api/core-test`); see [docs/WEB_UI.md](docs/WEB_UI.md), [docs/CORE_PARITY_GAPS.md](docs/CORE_PARITY_GAPS.md)
- [x] **MVP:** **OS login autostart** - `"autostart": "login"` in `dogecoinconf.json`; wizard finish checkbox + Settings → Interface toggle; `GET /api/autostart` status; Windows Task Scheduler · Linux systemd user / XDG autostart · macOS LaunchAgent (`autostart/`, `ui/autostart_sync.go`) - **S**
- [x] **MVP:** **config directory branding** - new installs save `dogecoinconf.json` under `%APPDATA%\DogeGo\` (legacy `%APPDATA%\dogego\` still read); `config.AppConfigDirName` - **S**
- [x] **MVP:** **wallet History UX (2026)** - card feed (not table); compact **confirmation count** badge; type chips (**Sent**, **Received**, **Mining**, **Quantum** + `pq_tag`); click row → detail sheet with **BlockStep** / block / address actions + copy txid/address (`ui/static/app.js`, `rpc/wallet_meta.go` `tx_kind`)
- [x] **Wallet History polish:** full **txid** on feed rows + **Export CSV** (History tab respects active filters; `GET /api/wallet/txs.csv` for full export) - **S** (`ui/wallet_txs_csv.go`)
- [x] **Wallet History polish:** **infinite scroll** - paginated `GET /api/wallet/txs?limit&offset&q&kind` (40 rows/page); debounced search + type filters server-side; scroll container + IntersectionObserver load-more; **Load all remaining** + auto-load when ≤200 txs; **soft refresh** patches confirmation badges when deep in scroll (`ui/static/app.js`, `rpc/wallet_ui_list.go`, `ui/wallet_txs_query.go`)
- [x] **Web UI polish:** **Send spendable balance card** - minimal card layout (replaces gold hero box) (`send-balance-card` in `index.html` / `app.css`)
- [x] **Web UI polish:** **dashboard API stability** - no overlapping `/api/live` polls, longer timeouts, higher failure threshold before disconnect banner (`ui/static/app.js`)
- [x] **Web UI polish:** **boot overlay** - dismiss when sync is healthy (+ wallet address when enabled); tx history loads in background (no longer blocks "Much prepare. Very load.") - **S** (`ui/static/app.js`)
- [x] **Web UI polish:** **BlockStep hero copy rows** - large centered txid/address + side copy button; 30s API timeout (`ui/static/blockstep.js`, `app.css`)
- [x] **Web UI polish:** **wallet History detail sheet** - BlockStep-style hero copy rows for address/txid (`wallet-tx-sheet-ids`)
- [x] **MVP:** **Send tab balance** - spendable DOGE headline + chips for pending / immature mining / UTXO count (`send-balance-card` in `index.html` / `app.css`)
- [x] **MVP:** **cross-platform operator cert** - `dogego cert offline` (no `.ps1` required); setup/docs de-emphasize PowerShell-only paths (`cmd/dogego/cert.go`; wallet web + RPC fast-path gates)
- [x] **Web UI polish:** **BlockStep** copy buttons on txid / address heroes, flow inputs/outputs, block tx cards (`ui/static/blockstep.js`, `app.css`) - **S**
- [x] **Operator UX:** `core_restart_resume_check.ps1` optional gate for OS autostart registration when `autostart=login` - **S** (`Test-DogeGoAutostartGate`, `GET /api/autostart` + Windows schtasks fallback; `ProbeCoreRestartResume` `os_autostart` check)
- [x] **Operator UX:** **`dogego cert autostart`** - cross-platform CLI check vs `dogecoinconf.json` (optional `-json`; shared `autostart.VerifyLogin`; included in `dogego cert offline` autostart package tests) - **S**
- [x] **Operator UX:** **reboot testnet founder playbook** - step-by-step checklist + sample founder/joiner `dogecoinconf.json` in `docs/OPERATOR.md` - **S**
- [x] **Operator UX:** **`dogego cert founder`** - cross-platform reboot testnet founder preflight (network, mine, inbound P2P, port 44556, fresh datadir hints; `founder/verify.go`; in `dogego cert offline`) - **S**
- [x] **Operator UX:** **`GET /api/core-founder-probe`** - 10th live web cert gate (`cert_founder`; mirrors `dogego cert founder`; skipped OK on mainnet) - **S**
- [x] **Operator UX:** **`GET /api/core-runner-probes`** - 11th live web cert gate (`runner_readiness`; dogego-live provision + preflight; workflow 10) - **S**
- [x] **Operator UX:** **setup wizard founder preflight** - `POST /api/setup/founder-preflight` on Finish step (testnet); Features **Founder probe** card - **S**
- [x] **Operator UX:** **founder probe strip pills** - Autostart + Founder mini-pills on Features live probe strip; `dogego cert founder -conf` / `-datadir`; wizard blocks **Save & start** on founder errors - **S**
- [x] **Wallet History polish:** detail sheet **Copy JSON** for integrators (`wallet-tx-sheet-copy-json`) - **S**
- [x] **Operator UX:** **OS login autostart cert gate** - 9th live web probe (`GET /api/core-autostart-probe`, cert matrix row `cert_autostart`; `/api/autostart` includes `verify`) - **S**
- [x] **Operator UX:** **probe strip polish** - clickable Features mini-pills jump to probe cards; `dogego cert autostart -conf`; setup wizard **Save & start** label shows founder warning count - **S**
- [x] **Operator UX:** **Features autostart probe card** - dedicated `feat-core-autostart` (mirrors `dogego cert autostart`; mini-pill + cert matrix jump target) - **S**
- [x] **Operator UX:** **`dogego cert provision`** - cross-platform dogego-live runner checklist (`runner/provision.go`; `-json` `-offline` `-preflight`; in `dogego cert offline`) - **S**
- [x] **Operator UX:** **`dogego cert preflight`** - cross-platform dogego-live RPC preflight (`runner/preflight.go`; `-json` `-offline` `-require-core` `-require-wallet-dat`; live path runs wallet.dat RPC probe or import when configured; mirrors `ci_runner_preflight.ps1`) - **S**
- [x] **Operator UX:** **setup wizard autostart preflight** - `POST /api/setup/autostart-preflight` on Finish when sign-in autostart enabled - **S**
- [x] **Operator UX:** **autostart registration soft-fail** - setup + Settings save return `ok: true` with `autostart_warning` when Task Scheduler / systemd registration fails (config still saved; node can start) - **S** (`ui/autostart_sync.go`, `ui/setup.go`, `ui/server.go`)
- [x] **Operator UX:** **`dogego open`** + **`dogecoin://` URL protocol** - `dogego open [--url URL]`; per-user `dogego register-url-protocol` (Windows registry / Linux xdg / macOS handler app); auto-registers on desktop start; maps `dogecoin://node` → dashboard, `dogecoin:ADDRESS` / BIP21 payment links → Send tab (`#send?to=…`) - **S** (`desktop/bip21.go`, `open_url.go`, `cmd/dogego/open.go`, `protocol.go`)
- [x] **Operator UX:** **system tray** - tray on by default on desktop (`config.TrayEnabled()`); `"tray": false` or `-tray=false` to disable; Open Dashboard / Shutdown Node menu; Windows minimize + close → tray; **official Dogecoin logo** embedded as `desktop/trayicon.png` (from `ui/static/dogecoin.svg`; Windows ICO via `desktop/trayicon_bytes_windows.go`) - **S** (`desktop/tray_run.go`, `cmd/dogego/tray_minimize_windows.go`, Settings + setup wizard)
- [x] **Web UI polish:** **Analytics dashboard** - bundled `rawblocks` stored-block count (`store/rawblock.go` `FastCount`); chart render when summary missing; `fmtDate` fix; **Disk breakdown** KPI title; modern top-UTXO address pills; analytics APIs match `/api/summary` LAN policy (no extra loopback gate on read-only `GET /api/analytics/*`) - **S**
- [x] **Operator UX:** **Console RPC tutorial** - `docs/RPC_CONSOLE_TUTORIAL.md`; Console tab cookbook (`GET /api/rpc/cookbook`, **Use in Console**); LAN `addnode` presets - **S**
- [x] **Operator UX:** **LAN peer pairing** - Settings P2P card (`GET /api/lan-peer-hint`, share target + one-click `addnode`); Overview **Pair LAN peer** shortcut; `scripts/lan_peer_pair.ps1`; `docs/OPERATOR.md` LAN section - **S** (`ui/lan_peer.go`, `ui/static/app.js`)
- [x] **Operator UX:** **P2P identity + node-tip HD key** - setup wizard Network step + Settings (`uacomment`, `uacomment_tip_address`, `uacomment_use_node_tip`); dedicated HD path `m/44'/3'/0'/2/0` published in wire user-agent; Receive address book `isnodetip` row; Overview user-agent line; live **`POST /api/config/uacomment-preview`** + Settings/wizard wire sub-version preview; `getaddressinfo`/`validateaddress` **`isnodetip`** - **S** (`config/uacomment.go`, `wallet/node_tip.go`, `ui/setup_uacomment.go`, `ui/uacomment_preview.go`)
- [x] **Operator UX:** **Settings autostart verify hint** - `/api/autostart` verify line + link to Features autostart probe card - **S**
- [x] **Operator UX:** **`GET /api/core-runner-probes`** + Features **CI runner readiness** card (`dogego cert weekly` bundle; `cli_weekly_live` / `cli_live_soak` hints) - **S**
- [x] **Operator UX:** **`dogego cert setup-parity`** - cross-platform reboottestnet Core parity bootstrap (`runner/setup_parity.go`; `-mine-bootstrap`; mirrors `setup_reboottestnet_core_parity.ps1`) - **S**
- [x] **Operator UX:** **`dogego cert weekly-live`** - cross-platform scheduled weekly CI bundle (`runner/weekly_live.go`; Core 24/24 + corruption mini; optional `-include-long-soak`) - **S**
- [x] **Operator UX:** **`dogego cert live-soak`** - cross-platform Milestone B multi-hour corruption soak (`runner/live_soak.go`; mirrors `ci_milestone_b_full_gate.ps1`) - **S**
- [x] **Operator UX:** **`dogego cert field-evidence`** - cross-platform Milestone A mainnet field evidence gates (`fieldevidence/suites.go`; mirrors `field_evidence_cert.ps1`) - **S**
- [x] **Operator UX:** **`dogego cert weekly`** - one-shot dogego-live weekly CI gate (`provision -preflight` + `preflight -require-core`; optional `-json`; live `wallet.dat` RPC import only when explicitly configured or `-require-wallet-dat`) - **S**
- [x] **Milestone E (operator workflow certification, partial):** cross-platform weekly live readiness (`dogego cert weekly`; complements `gh_enable_scheduled_live.ps1` on dogego-live)
- [x] **Operator UX:** **`dogego cert enable-weekly`** - cross-platform `gh variable set` for scheduled live CI (`runner/enable_weekly.go`; `-weekly-only` `-require-wallet-dat` `-dry-run` `-repo`; sets optional `DOGEGO_WALLET_DAT_REQUIRED`) - **S**
- [x] **Operator UX:** **`dogego cert milestones-bde`** - offline close for milestones B/D/E code gates (crash+corpus+operator; live soak still on dogego-live) - **S** (`runner/milestones_bde.go`, `cmd/dogego/cert_milestones_bde.go`)
- [x] **Operator UX:** **`dogego cert workflow10`** - one-shot dogego-live workflow 10 orchestrator (`runner/workflow10.go`; optional `-enable-github` dry-run/apply → `provision -preflight -run-setup` → `weekly-live` → optional `-include-live-soak`; `-stop-after provision|weekly-live`; `GET /api/core-runner-probes` `cli_workflow10`) - **S**
- [x] **Operator UX:** **`GET /api/core-workflow10-probe`** - Features workflow 10 preflight (`dogego cert workflow10 -skip-scripts`; stage list; `ui/workflow10_probe.go`) - **S**
- [x] **Operator UX:** **Features workflow 10 polish** - auto-load on Features tab; probe strip mini-pill; included in **`GET /api/core-probes`** bundle - **S**
- [x] **Operator UX:** **Features runner auto-load** + Settings/Overview **CI runner** links (`feat-core-runner`) - **S**
- [x] **Web UI polish:** **wallet UTXO-cache fast path** - `/api/wallet` balance/immature/UTXO count from live cache (BlockStep parity); `/api/wallet/txs` + **`/api/wallet/txs.csv`** + **`/api/wallet/utxos`** (Send coin control) without blocking RPC; async wallet poll does not stall Overview refresh (`ui/wallet_fast.go`, `wallet_api.go`, `ui/static/app.js`; `ui/wallet_fast_test.go`)
- [x] **Web UI polish:** **Send coin control (Advanced)** - `/api/wallet/utxos` matches all wallet **SpendScripts** (HD receive/change), not only default address; clears loading spinner + shows fetch/unlock errors; first **300** UTXOs listed with total hint (`TestWalletListUnspentFromUtxoCacheAllSpendScripts`)
- [x] **Web UI polish:** **Send fee hints** - `/api/wallet/send` returns `fee_hint`, `suggested_fee_rate`, `estimated_fee_doge` on fee-related errors; one-click retry in Send tab (`ui/wallet_send_hints.go`, `ui/static/app.js`)
- [x] **Web UI:** **Receive address book** - `GET /api/wallet/addresses` sorted by HD path/type (`wallet/address_sort.go`); `GET /api/wallet/labels` (`listlabels`); toolbar filter + type filter + label datalist; **Generate address** (`POST /api/wallet/address/new` → `getnewaddress`); inline label edit (`POST /api/wallet/address/label` → `setlabel`); per-row copy; stable refresh (no poll flash); split `#recv-meta` / `#recv-status` (`ui/wallet_import.go`, `ui/static/app.js`, `ui/wallet_import_test.go`)
- [x] **Operator doc:** solo-mining wallet performance - dashboard UTXO-cache vs `tx_index_embed_tx` for Console RPC (`docs/OPERATOR.md`, `docs/WEB_UI.md` Send/History notes)
- [x] **Wallet probe:** `/api/core-wallet-probe` exposes `spendable_utxo_count`, `address_book_count`, `address_book_node_tip_count`, `nodetip_validateaddress_ok`, `nodetip_getaddressinfo_ok`, `label_roundtrip_ok`, `label_list_ok` from wallet RPC; optional **`dogego_probewalletdat`** when **`DOGEGO_WALLET_DAT`** is set (pool metadata + **`pool_keys_unmatched`** warning + **`pool_indices_replayed`** + **`pool_replay_scan_cap`** when **`pool_count`>0 + **`keypool_hint`**) (`ui/core_wallet_probe.go`; golden operator RPC wallet batch in `operator_rpc_errors_test.go`)
- [x] **Operator doc:** Core wallet.dat migration - probe/import/cert table in `docs/CORE_OPERATOR_RUNBOOK.md` (`pool_keys_matched`/`pool_keys_unmatched`, `wallet/pool_replay.go`, `dogego cert wallet-migration`)
- [x] **CI offline gate:** wallet web cert includes `TestProbeCoreWallet`, `TestProbeCoreAddrman`, wallet txs defer (`TestWalletTxHistoryDefer*`); **wallet.dat migration** synthetic BDB fixtures (`walletmigration/`, `TestExecImportWalletDatNativeSyntheticFixture`); connect catch-up + addrman soak + `TestCoreMainnetFieldMultiTxBlock15504` + summary/RPC boost diagnostics - single suite list in **`offlinegate/suites.go`** + testdata bootstrap in **`offlinegate/bootstrap.go`**; field evidence in **`fieldevidence/suites.go`**; wallet migration offline gates in **`walletmigration/verify.go`** (`ci_offline_gate.sh` / `.ps1`, `dogego cert offline`, `docs/scripts_cert_test.go` drift guards)
- [x] **Operator UX:** **Console RPC cookbook example params** - **`GET /api/rpc/cookbook`** includes per-method **`params[]`** with placeholders (`WALLET`, `TXID`, `HOST:44556`); **Use in Console** applies params via **`substituteRPCParams`** (`rpc/cookbook_examples.go`, `ui/static/app.js`) - **S**
- [x] **Web UI polish:** **boot overlay hard refresh** - **`LiveFeed.bootstrapLiveIfEmpty`** immediate `/api/live`; relaxed **`bootReadyToShow`** (+ max overlay timeout); sync-health dismiss during IBD (`ui/live.go`, `ui/static/app.js`) - **S**
- [x] **Operator UX:** **setup wizard optional wallet encryption** - **`POST /api/setup/apply`** **`wallet_encrypt`** + **`wallet_passphrase`** on finish when wallet enabled (`ui/setup_wallet_encrypt.go`, `setup.html`) - **S**
- [x] **Operator UX:** **web wallet passphrase unlock** - **`POST /api/wallet/unlock`** / **`POST /api/wallet/lock`** (loopback); modal separate from dashboard PIN; Send **Unlock & send**; **`private_keys_enabled`** + **`unlocked_until`** on **`GET /api/wallet`** (`ui/wallet_unlock.go`, `wallet_passphrase.js`) - **S**
- [x] **Web UI polish:** **Analytics connected peers** - Core-like inbound/outbound table with sync role, ping, traffic, DGR flag; **`GET /api/peers`** (`ui/peers_api.go`, `node/peerinfo_rpc.go`) - **S**
- [x] **Operator UX:** **Settings → Services** - runtime start/stop/restart for analytics sidecar + **solo mining**; contextual action buttons per service state (`node/runtime_services.go`, `node/solo_mining_runtime.go`, `ui/static/app.js`) - **S**
- [x] **Operator UX:** **DGR settings UX** - role cards (Off / Client / Operator); multi-line **`relay_dnsseed`**; wizard/preflight auto-role; fix load overwrite of saved outbound config (`index.html`, `config/validate.go`, `node/dgr/discover.go`, `app.js`) - **S**
- [x] **Web UI polish:** **Settings wallet encryption panel** - lock/unlock spend keys + **`unlocked_until`** countdown (distinct from dashboard PIN; Settings → Wallet) - **S**
- [x] **Operator UX:** **Settings → Tools** - categorized RPC catalog from **`GET /api/rpc/cookbook`** with search, Run, and Open in Console; **`st-tools-groups`** panel (`ui/static/index.html`, `ui/static/app.js`) - **S**
- [x] **Operator UX:** **Settings wallet encrypt/change passphrase** - **`POST /api/wallet/encrypt`**, **`POST /api/wallet/passphrase-change`** from Settings → Wallet (`ui/wallet_encrypt_api.go`) - **S**
- [x] **Operator UX:** **node restart reliability** - detached replacement spawn, **`-waitpid`**, preserve CLI args (`cmd/dogego/restart_spawn*.go`, `store/process_wait.go`, `POST /api/control/restart`) - **S**
- [x] **Operator UX:** **uacomment / node-tip UX** - auto HD node-tip when wallet enabled; manual tip address only when wallet disabled; wizard aligned (`ui/uacomment_preview.go`, `setup.html`) - **S**
- [x] **Operator UX:** **system tray v2** - running version line, Open Console, View activity logs, check/download/dismiss updates (`desktop/tray_run.go`, `desktop/open_url.go`) - **S**
- [x] **Operator UX:** **auto-update** - daily GitHub Releases check, Overview banner + **Install update** button, tray menu, native OS notification, **`POST /api/update/download`** with SHA256 verify, **`POST /api/update/apply`** install+restart, **`-replacetarget`** install path swap, **`dogego version -check`**, **`scripts/check_update.ps1`** / **`check_update.sh`** (`version/updatecheck.go`, `version/update_download.go`, `ui/update_api.go`, `desktop/notify*.go`) - **S**
- [x] **Operator UX:** **GitHub release workflow** - tag **`v*`** builds windows/linux/darwin binaries + **`.sha256`** sidecars (`.github/workflows/release.yml`) - **S**
- [x] **Operator UX:** **monthly wallet backup reminder** - Receive tab banner when no **`dogego_backup_last_download`** in 30 days; dismiss snoozes 30 days (`wallet-backup-remind`, `ui/static/app.js`) - **S**
- [x] **Phase 12:** **auto-update operator docs** - [OPERATOR.md](docs/OPERATOR.md) § Auto-update, [WEB_UI.md](docs/WEB_UI.md), [INTEGRATION.md](docs/INTEGRATION.md), [DEVELOPER_GUIDE.md](docs/DEVELOPER_GUIDE.md) - **S**
- [x] **Operator UX:** **Settings → Interface updates panel** - **`st-update-status`**, **Check now** (`POST /api/update/check`), download / install / dismiss (`ui/static/index.html`, `app.js`) - **S**
- [x] **Operator UX:** **`dogego version -check` / `-json`** - CLI update probe for scripts and CI (`cmd/dogego/main.go`) - **S**
- [x] **Operator UX:** **dogego-live workflow 10 runbook** - OPERATOR.md quick start + Features probes (`dogego cert workflow10`, `enable-weekly`) - **S**
- [x] **Operator UX:** **native OS update notification** - desktop balloon (Windows), `notify-send` (Linux), Notification Center (macOS) on newly detected release (`desktop/notify*.go`, `version.UpdateChecker.SetOnAvailable`) - **S**
- [x] **Operator UX:** **`scripts/check_update.ps1` / `check_update.sh`** - wrapper for `dogego version -check` / `-json` - **S**
- [x] **Operator UX:** **`scripts/schedule_update_check.ps1` / `schedule_update_check.sh`** - daily Task Scheduler / cron helper for offline update checks - **S**
---

## Phase 12 - Documentation & integrator UX

**Goal:** Anyone can learn **how DogeGo works**, **how to operate it**, **how to call every RPC**, and **how to connect external apps** - from **`docs/`** and the **web UI** (Guide, Docs, Features), kept in sync with code.

### Repo documentation (`docs/`)

- [x] **MVP:** [docs/DOCUMENTATION.md](docs/DOCUMENTATION.md) - master index + contributor checklist
- [x] **MVP:** [docs/INTEGRATION.md](docs/INTEGRATION.md) - JSON-RPC auth, ports, curl/Python examples, dashboard `/api/*`
- [x] **MVP:** [docs/RPC.md](docs/RPC.md) - workflow index (chain, mempool, mining, PSBT, network)
- [x] **MVP:** [docs/WALLET.md](docs/WALLET.md) - built-in wallet workflow index
- [x] **MVP:** existing [docs/OVERVIEW.md](docs/OVERVIEW.md), [docs/OPERATOR.md](docs/OPERATOR.md), [docs/WEB_UI.md](docs/WEB_UI.md), [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/SECURITY.md](docs/SECURITY.md), [docs/INTENTIONAL_DIFFERENCES.md](docs/INTENTIONAL_DIFFERENCES.md)
- [x] **Per-RPC cookbook** - copy-paste `curl` / CLI example for every `SupportedMethods()` entry with example **`params[]`** placeholders (`GET /api/rpc/cookbook`, `rpc/cookbook_examples.go`) - **L**
- [x] **Auto-generated RPC reference** from `help.go` + parameter schemas (HTML) - **M** (`GET /api/rpc/reference.html`)
- [x] **OpenAPI** (or OpenRPC) description for JSON-RPC surface (`GET /api/openrpc.json`) - **M**
- [x] **Integration guides** by language (Go, Python, Node, Rust) with auth + error handling - **M** (`GET /api/integration/guides`)
- [x] **Exchange / pool operator runbook** (mainnet warnings, Core vs DogeGo decision tree) - **S**; see [docs/CORE_OPERATOR_RUNBOOK.md](docs/CORE_OPERATOR_RUNBOOK.md)
- [x] **MVP:** [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) - PR checklist (RPC, docs, ROADMAP, tests)

### Web UI documentation

- [x] **MVP:** **Docs** tab - `GET /api/docs` (`ui/docs_index.go`); topics: map, external apps, implement/extend, RPC, wallet, web UI, storage
- [x] **MVP:** **Features** tab - live searchable RPC catalog (`SupportedMethods` + `help.go` classification)
- [x] **MVP:** **Guide** tab - operator concepts (sync, P2P, mempool, wallet, security)
- [x] **Docs tab:** search across manifest sections (`docs-search` filter) - **S**
- [x] **Docs tab:** render embedded `docs/*.md` in-browser via `GET /api/docs/md` + marked (`ui/docs_embed.go`, Docs tab viewer)
- [x] **Settings / Overview:** Documentation callout with links to Docs / Guide / Console - **S**
- [x] **Setup wizard:** profile step + preflight firewall/CGNAT checks + finish step (`setup.html`, `/api/setup/preflight`) - **S**
- [x] **Per-tab help** collapsible panels on Send / Explorer / Mempool (`index.html`, WEB_UI.md summaries)
- [x] **Full node without HTTP RPC:** chain `DataPaths` + web Console + header recovery wired when `node_mode=full` even if `rpc` listen is empty (`node/run.go`)

### Keep in sync (process)

- [x] **MVP:** CI check: `rpc/dispatch_sync_test.go` + `help_test.go` - every `dispatch` case and `SupportedMethods()` entry must have `help.go` text
- [x] **MVP:** CI check: `ui/docs_index_test.go` - Docs manifest mentions wallet/mempool workflow RPCs
- [x] **Quarterly doc audit vs `ROADMAP.md` checked items** - **S** (`docs/doc_audit_test.go`)

---

## Phase 9 - Security hardening

- [x] **MVP:** **TLS / remote access** documented - reverse-proxy pattern + optional native PEM TLS in [docs/OPERATOR.md](docs/OPERATOR.md)
- [x] **MVP:** Native TLS for RPC + UI (optional PEM paths in `dogecoinconf.json`; reverse proxy still recommended for production)
- [x] **MVP:** **Wallet encryption at rest** (`wallet/crypto.go`, `wallet/encrypt.go`) - not interchangeable with Core `wallet.dat` format.
- [x] **MVP:** [docs/SECURITY.md](docs/SECURITY.md) threat-model summary + operator checklist
- [x] **Optional (Phase 9):** Formal audit vs Core deployment guides - **L** (third-party review before high-value production claims; not a feature gap).

---

## Phase 10 - Post-quantum commitments & libdogecoin alignment

- [x] Track **draft PQ signature commitments** spec (Phase 1 `FLC1`/`DIL2`/`RCG4`; RPC `dogego_verifypqcommitment` off-chain verify). Upstream: [libdogecoin PQC carrier spec](https://github.com/edtubbs/libdogecoin/blob/0.1.5-dev-pqc-carrier/doc/spec/bip-post-quantum-signature-commitments.mediawiki).
- [x] **Recognition** tooling - canonical `OP_RETURN` PQ tags (`FLC1` / `DIL2` / `RCG4`) in `consensus/pq_commitment.go`; `decodescript` / `getblock` `scriptPubKey` expose `dogego_pqc_*` fields (no verify).
- [x] **`createrawtransaction`** `pqcommit` output (`tag` + 32-byte `commitment` hex) via `BuildPQCommitmentScript`.
- [x] Web UI **wallet flags** (`pq_commitments`, `avoid_reuse`) + optional PQ OP_RETURN on **Send** tab (`/api/wallet/flags`, `/api/wallet/send`).
- [x] **Verification path** (off-chain format + pqcrypto verify) - `dogego_verifypqcommitment` / `dogego_verifypqcarrier` + `consensus.VerifyPQCommitmentScriptHex` / `VerifyPQCarrierPair`; offline **`dogego cert pq`** (`pqcert/`); live **`GET /api/core-pq-probe`**; mempool relay standardness for PQ OP_RETURN + carrier P2SH - shipped (more soak testing welcome; not consensus-enforced)
- [x] **Phase-1 TX_C/TX_R P2SH carrier** (`consensus/pq_carrier.go`, `pqcrypto/`, `dogego_createpqcarrier`, `dogego_sendpqcarrier`, `dogego_verifypqcarrier`) - format + sign/verify metadata; Raccoon-G vendors [libdogecoin `src/raccoon_g`](https://github.com/dogecoinfoundation/libdogecoin/tree/0.1.5-dev/src/raccoon_g) / [Core PR #8](https://github.com/dogecoinfoundation/dogecoin/pull/8); **no consensus fork**
- [x] **Wallet flags** **`pq_commitments_enabled`** + **`pq_carrier_enabled`** + separate PQ key storage (`wallet/pq_carrier.go`); Settings → Wallet toggles; Send → Advanced carrier mode (`pq_mode: carrier`); History detects `carrier_scriptsig` via `walletPQMetaFromTxHex`
- [x] PQ ships for testing; more soak welcome; not a consensus-enforced PQ softfork.

---

## Phase 13 - Extensions & optional ZK L2

Optional **extensions** (no mainnet consensus fork). Third-party packages ship `dogego.extension.json`; install from zip/GitHub; enable/disable via RPC and web UI. Extensions **never** receive wallet, private key, or signing APIs.

- [x] **MVP:** Extension manifest v1 + permission denylist (`extensions/manifest.go`)
- [x] **MVP:** Registry install/list/enable/disable + zip install with path traversal guards (`extensions/registry.go`, `zip_install.go`)
- [x] **MVP:** Sandbox host API - chain read + extension datadir only (`extensions/host.go`, `chain_adapter.go`)
- [x] **MVP:** RPC `dogego_listextensions`, `dogego_enableextension`, `dogego_disableextension`, `dogego_instextensionzip` (`rpc/extensions_rpc.go`)
- [x] **MVP:** Web UI `GET /api/extensions`, `POST /api/extensions/enable|disable` (`ui/extensions_api.go`)
- [x] **MVP:** Built-in **`dogego.zkl2`** / P2P **`zkproof-v1`** - tx+block-anchored proofs, off-L1 `VerifyCheckZKP`, Pebble store, ProofRoot, `exthello`/`extack`, `zkinv`/`getzkproof`/`zkproof` (`extensions/zkl2/`, `extensions/p2p.go`, `node/ext_p2p.go`)
- [x] **MVP:** Docs - [docs/EXTENSIONS.md](docs/EXTENSIONS.md), [extensions/catalog/zkl2/docs/PROTOCOL.md](extensions/catalog/zkl2/docs/PROTOCOL.md), [extensions/catalog/zkl2/docs/USER_GUIDE.md](extensions/catalog/zkl2/docs/USER_GUIDE.md), [extensions/catalog/AUTHORING.md](extensions/catalog/AUTHORING.md)
- [x] **Catalog + lifecycle** - GitHub `catalog.json`, `dogego_listextensioncatalog`, install URL/catalog/uninstall RPC, zip upload UI (`extensions/catalog.go`, `ui/extensions_api.go`)
- [x] **ZK overlay sync** - `getzkheaders`/`zkheaders`, `getzkblockproofs`, background inv relay, peer overlay registry, live L1 anchor index
- [x] **Extensions catalog UI** - Settings → Extensions tab (`ui/static/index.html`, `app.js`)
- [x] **ZKPG wire format** - structured Groth16 blob validation; compressed 192 B + DIP 384 B pairing verify with `data/vk/*.vk`
- [x] **Catalog release pins** - `download_url` + `sha256` for `example.go` / `example.wasm` in `extensions/catalog/catalog.json`
- [x] **Subprocess host** - sandboxed `entry.binary` line JSON-RPC (`extensions/subprocess_ext.go`, [SUBPROCESS_PROTOCOL.md](extensions/catalog/SUBPROCESS_PROTOCOL.md))
- [x] **Wasm host** - execute `entry.wasm` via wazero (export-per-RPC, `dogego.log` import)
- [x] **Runtime permission enforcement** - scoped host checks chain/datadir/p2p permissions per extension call
- [x] **Relay peer extension negotiate** - `exthello`/`extack` on relay/inbound peers; overlay dispatch + disconnect cleanup
- [x] **Groth16 pairing verify** - BLS12-381 compressed proof (192 B) + DIP affine proof (384 B) + snarkjs VK (`data/vk/*.vk`) via circl `ProdPairFrac`
- [x] **Example extension build scripts** - `extensions/catalog/example-go/build.{ps1,sh}`, `example-wasm/build.{ps1,sh}`; catalog `sha256` pin workflow documented
- [x] **Inline DIP VK** - `verifying_key` or `verifying_key_chunks` (6×80 B) on verifyproof/checkzkp; `JoinDIPVKChunks` for #3869 stack layout

---

## Phase 11 - Wallet beyond testnet stub

- [x] **MVP:** Built-in testnet wallet subset: `getnewaddress`, `listunspent`, `getbalance`, `dumpprivkey`, **`importprivkey`**, `fundrawtransaction`, `signrawtransaction`, **`sendtoaddress`**, **`sendfrom`**, **`sendmany`** (P2PKH via UTXO cache); **`subtractfeefromamount`** / **`subtractfeefrom`**; **`listsinceblock`**, **`listtransactions`**, **`gettransaction`**; **`signmessage`**; **web Send** + **`GET /api/wallet/txs`**
- [x] **MVP:** `fundrawtransaction` inputs use **BIP125** `nSequence` (wallet sends replaceable in mempool)
- [x] **MVP:** `bumpfee` auto-bump for built-in wallet (reduces change output, re-signs) or `options.rawtx`; optional `fee_rate`
- [x] **MVP:** Watch-only **`importaddress`** / **`importpubkey`** (P2PKH in `wallet.json`; `listunspent` / `getbalance` / `listreceivedbyaddress` / `validateaddress` `iswatchonly`)
- [x] **MVP:** **`getunconfirmedbalance`** / **`getwalletinfo`** `unconfirmed_balance` from mempool (tracked scripts)
- [x] **MVP:** **`backupwallet`** copies `wallet.json` when the built-in wallet is enabled
- [x] **MVP:** **`dumpwallet`**, **`listaccounts`**, **`getaddressesbyaccount`**, **`listaddressgroupings`** for built-in wallet
- [x] **MVP:** **`validateaddress`** P2SH; **`importaddress`** P2SH address + `p2sh` redeem script
- [x] **MVP:** **`validateaddress`** / **`getaddressinfo`** optional **`redeemScript`** (P2SH hash check, `isscript`, redeem metadata)
- [x] **MVP:** **`signrawtransaction`** P2SH timelocks - **CLTV/CSV + P2PKH/multisig** (`BuildCLTV*`, `BuildCSV*` redeem helpers)
- [x] **MVP:** **`decodescript`** bare **multisig** redeem (`type`/`reqSigs`/`addresses`); **`fundrawtransaction`** watch P2SH inputs + **P2SH change** outputs
- [x] **MVP:** **`wallet.json` `watch_redeems`** - P2SH multisig redeem persisted; **`buildWalletPrevTxs`** / auto **`signrawtransaction`** prevouts include **`redeemScript`**
- [x] **MVP:** **`settxfee`** persisted in `wallet.json`; **`fundrawtransaction`** uses wallet paytxfee
- [x] **MVP:** **`rescan`** succeeds for built-in wallet (UTXO-cache model; no separate wallet tx index)
- [x] **MVP:** **`importwallet`** / **`dumpwallet`** round-trip for WIF + watch scripts + **`redeem=1`** P2SH redeem lines (`watch_redeems`); **`descriptor=1`** lines for known import descriptors; **`getaddressinfo`** / **`validateaddress`** **`desc`** when wallet knows descriptor
- [x] **MVP:** **`importprivkey`** cosigner keys in **`extra_privkeys_hex`**; **`signrawtransaction`** / **`signrawtransactionwithwallet`** / **`sendtoaddress`** sign P2SH multisig when enough wallet WIFs match the redeem script (auto prevouts + redeem from **`watch_redeems`**)
- [x] **MVP:** **`importprunedfunds`** - CMerkleBlock proof + header-chain check; credits built-in wallet watch outputs (`wallet.json` `pruned_imports`; `listtransactions` / `listsinceblock`)
- [x] **MVP:** **`listreceivedbyaccount`** / **`getreceivedbyaccount`** for default account
- [x] **MVP:** **`lockunspent`** / **`listlockunspent`** (persisted); **`fundrawtransaction`** uses wallet UTXOs only + skips locked
- [x] **MVP:** **`getaccountaddress`**, **`liststucktransactions`** for built-in wallet
- [x] **MVP:** **`addmultisigaddress`** imports P2SH multisig watch-only; **`keypoolrefill`**; **`getwalletinfo`** `txcount`
- [x] **MVP:** **`getaddressinfo`** (mine/watch, pubkey for spend key); **`getaddressinfo`** / **`validateaddress`** auto-load P2SH **`watch_redeems`** + **`solvable`** when enough cosigner keys for imported **`sh(multi)`**; **`importmulti`** watch-only batch; **`move`** no-op when wallet enabled
- [x] **MVP:** **`listlabels`** / **`setlabel`** in `wallet.json`; **`signrawtransactionwithwallet`**; **`setaccount`** for tracked addresses; unencrypted **`walletpassphrase`** errors
- [x] **MVP:** **`importmulti`** `pubkeys` + `required` multisig watch import
- [x] **MVP:** **`getbalances`**, **`listreceivedbylabel`** / **`getreceivedbylabel`**; wallet-scoped **`resendwallettransactions`**
- [x] **MVP:** Wallet-scoped **`abandontransaction`** (persist **`abandoned_txs`** in `wallet.json`; **`abandoned`** on list/get tx); **`removeprunedfunds`** drops abandoned or **`pruned_imports`** records; **`listtransactions`** label / watch filters; **`importaddress`** label; **`listunspent`** `label` field
- [x] **MVP:** **`getnewaddress`** label; **`importmulti`** / **`importpubkey`** labels; Core-shaped **`getaddressesbylabel`** address object (`purpose=receive|send` for change); **`getwalletinfo`** `walletname` / `format` / **`private_keys_enabled`**
- [x] **MVP:** HD **`getaddressesbyaccount`** / **`getaccount`** / label RPCs enumerate all issued receive+change indices; **`signrawtransactionwithwallet`** / **`dumpprivkey`** / **`signmessage`** require unlock when encrypted; **`importprivkey`** / **`importaddress`** / **`importpubkey`** optional rescan (SyncUtxo + block scan)
- [x] **MVP:** **`getaddressinfo`** / **`validateaddress`** **`hdkeypath`** / **`pubkey`** for wallet addresses; **`importmulti`** **`options.rescan`**; **`bumpfee`** / **`getnewaddress`** / **`keypoolrefill`** / **`fundrawtransaction`** / **`importmulti` keys** wallet-lock **`-13`**
- [x] **MVP:** **`getaccountaddress`** returns HD keypool peek; **`listaddressgroupings`** groups by address; **`getwalletinfo`** **`hdchainid`** (BIP44 coin type 3)
- [x] **MVP:** **`importprivkey`** label + wallet-lock **`-13`**; **`signrawtransaction`** lock when using wallet keys; **`getbalances`** / **`getwalletinfo`** HD spend scripts + **`watchonly_balance`**
- [x] **MVP:** **`listunspent`** **`query_options`** (`minimumAmount`, `maximumCount`, …); **`importwallet`** rescan + dump **`label=`** lines; **`getwalletinfo`** **`keypoololdest`** from wallet file mtime
- [x] **MVP:** **`getbalance`** **`include_watchonly`**; **`listunspent`** / **`listtransactions`** **`iswatchonly`** + **`blocktime`** on confirmed txs
- [x] **MVP:** **`listunspent`** **`include_unsafe`** + Core-shaped **`safe`**; **`getwalletinfo`** **`keypoolsize_hd_external`** / **`keypoolsize_hd_internal`**; **`gettransaction`** / **`liststucktransactions`** send **`fee`** via HD spend scripts + tx index prevouts
- [x] **MVP:** **`hdseedid`** on **`getwalletinfo`** (HD unlocked); coinbase **maturity** from **`LookupConsensus`** on **`getwalletinfo`** / **`getbalances`** / **`listunspent`** (`safe`, immature, spendable)
- [x] **MVP:** **`listtransactions`** / **`listsinceblock`** send rows include **`fee`** (same path as **`gettransaction`**)
- [x] **MVP:** **`listsinceblock`** **`include_watchonly`** + **`target_confirmations`**; **`fundrawtransaction`** auto **`changeAddress`** (HD change) and skip **immature coinbase** inputs
- [x] **MVP:** **`fundrawtransaction`** **`replaceable`** option; **`bumpfee`** HD change output detection; **`gettransaction`** multi-output **`details`**; **`abandontransaction`** HD spend detection
- [x] **MVP:** **BIP44 HD** (`m/44'/3'/0'/0/n` receive, `…/1/n` change) in `wallet.json` (`hd_seed_hex`, keypool); **`getnewaddress`** / **`getrawchangeaddress`** / **`keypoolrefill`**; UTXO scan + **`fundrawtransaction`** / **`sendtoaddress`** across issued indices; **`dumpprivkey`** per address
- [x] **MVP:** Wallet encryption at rest - **`encryptwallet`**, **`walletpassphrase`**, **`walletlock`**, **`walletpassphrasechange`** (scrypt + AES-GCM; **`getwalletinfo`** `encrypted` / `unlocked_until`; auto-lock on timeout)
- [x] **MVP:** Built-in wallet on **mainnet** and testnet (`wallet.json` per `datadir/<network>/`; BIP44 coin type 3)
- [x] **MVP:** **`rescan`** refreshes UTXO cache via **`SyncUtxo`** when full node (`wallet_rescan.go`)
- [x] **MVP:** HD keypool **auto top-up** on startup and after **`getnewaddress`** when pool &lt; half target (`ensureKeypoolLocked`, `EnsureKeypoolOnLoad`; `hd_keypool` in `wallet.json`)
- [x] **MVP:** **`fundrawtransaction`** **`conf_target`** / **`estimate_mode`** / **`fee_rate`** options (smart fee when no explicit rate)
- [x] **MVP:** **`fundrawtransaction`** **`lockUnspents`** / **`includeWatching`** / **`minimumTotalFee`** (Core defaults: lock on, watch off)
- [x] **MVP:** **`getreceivedbyaddress`** **`include_watchonly`**; **`sendtoaddress`** optional fund **options** object (`fee_rate`, `conf_target`, …)
- [x] **MVP:** **`listreceivedbyaddress`** **`iswatchonly`**; **`getwalletinfo`** **`scanning`** during rescan; **incremental wallet rescan** on node startup when scan history lags raw tip
- [x] **MVP:** HD **internal change keypool** (`hd_change_keypool`, `ChangeKeypoolSize`, auto top-up; **`getwalletinfo`** **`keypoolsize_hd_internal`**; **`keypoolrefill`** fills receive + change pools)
- [x] **MVP:** **`sendfrom`** / **`sendmany`** optional trailing **fund options** JSON (`fee_rate`, `conf_target`, `replaceable`, …)
- [x] **MVP:** **`fundrawtransaction`** **`add_inputs`** option (default true; false = fund fee/change using existing inputs only)
- [x] **MVP:** **`getdescriptorinfo`** / **`importdescriptors`** watch-only **`pkh`** / **`sh(pkh)`** subset; **`estimaterawfee`** RPC; **`gettransaction`** **`involvesWatchonly`** + detail **`iswatchonly`**
- [x] **MVP:** **`signrawtransactionwithkey`** (explicit WIF keys, no wallet merge); **`importdescriptors`** optional **`keys`** spendable import; **`getdescriptorinfo`** **`issolvable`** / **`hasprivatekeys`** when wallet holds key
- [x] **MVP:** **`importdescriptors`** **`internal`** / **`timestamp`** persisted in **`wallet.json`** (`imported_descriptors`); **`listdescriptors`** merges import metadata; **`getaddressinfo`** **`reused`** when **`avoid_reuse`** enabled
- [x] **MVP:** **`importmulti`** **`internal`** / **`timestamp`** → same **`imported_descriptors`** index; **`importmulti`** / **`importdescriptors`** **`desc`** (`pkh`, `sh(pkh)`, **`sh(multi)`**, **`sh(cltv(N)multi)`** / **`sh(csv(N)multi)`**, **`sh(cltv(N)pkh)`** / **`sh(csv(N)pkh)`**, bare **`multi`** when **`permitbaremultisig`**); **`importdescriptors`** **`keys[]`** validated against descriptor (Core: key must appear in `desc`); **`listdescriptors`** emits matching descriptors from watch redeems; **`getaddressinfo`** **`solvable`** for CLTV/CSV P2SH multisig when enough cosigner keys; **`fundrawtransaction`** / **`sendtoaddress`** select **solvable** watch multisig without **`includeWatching`**; wallet sign/send sets **`nLockTime`** (CLTV) and input **`nSequence`** / tx **version ≥2** (CSV) for timelock P2SH spends; **`getblockfilter`** BIP158 **basic**
- [x] **MVP:** **`walletcreatefundedpsbt`** / **`walletprocesspsbt`** - fund via wallet UTXO cache + **`fundrawtransaction`** options; sign with **`final_scriptSig`** (P2PKH / P2SH multisig subset; no BIP32 deriv paths)
- [x] **MVP:** **`listdescriptors`**; **`setwalletflag`** **`avoid_reuse`** (+ **`getwalletinfo.avoid_reuse`**); **`PeekChangeAddress`** / **`CommitChangeAddress`** (fund no longer drains change keypool on every peek)
- [x] **MVP:** **`deriveaddresses`** / **`extractdescriptor`** for supported output descriptors (non-range subset); **`importmulti`** **`desc`** round-trip for **`sh(cltv/csv)pkh`**; wallet **`bumpfee`** applies CLTV locktime on replacement tx
- [x] **MVP:** **`addr()`** output descriptor (watch/import/derive; maps to P2PKH like Core)
- [x] **MVP:** **`gettxspendingprevout`** - mempool scan with Core-shaped **`spendingtxid`** per outpoint
- [x] **MVP:** **`decodepsbt`** - BIP-174 parse to JSON (hex or base64; legacy unsigned tx; no witness PSBT)
- [x] **MVP:** **`scantxoutset`** - scan in-memory UTXO cache for **`addr`/`pkh`/`sh(pkh)`/`sh(multi)`/`multi`/`raw`** descriptors (`start`/`abort`/`status`; synchronous; requires UTXO synced to chainActive)
- [x] **MVP:** **`dogego_importmnemonic`** - BIP39 HD restore (`m/44'/3'/0'/0/0` mainnet golden); **`dogego_importbip38`** - BIP38 decrypt (non-EC, EC-multiply, lot/sequence); **`dogego_listwalletaddresses`**; UI `/api/wallet/import` + address book; offline **`dogego cert wallet-import`** (`walletimport/verify.go`; mirrors `scripts/wallet_import_cert.ps1`)
- [x] **MVP:** **`analyzepsbt`** / **`combinepsbt`** / **`joinpsbts`** / **`utxoupdatepsbt`** / **`finalizepsbt`** / **`createpsbt`** / **`converttopsbt`** - PSBT utility RPC subset (legacy; prevouts from txindex + rawblocks + mempool)
- [x] **MVP:** block filter index **deferred until contiguous raw chain** (quieter IBD; backfill on `chainActive` advance + `reindexblockfilters`)
- [x] **MVP:** **`avoid_reuse`** coin selection deprioritizes reused receive scripts; **`listunspent`** **`reused`** when flag enabled (`wallet/reused.go`, `fundrawtransaction` sort)
- [x] **Done:** Core-like keypool in HD `wallet.json` (receive/change pools, `keypoolrefill`, `iskeypool`, consume on spend) + wallet.dat import/pool replay (`hd_keypool_core_index`). Same behavior as Core’s pool; JSON storage instead of wallet.dat BDB.
- [x] **MVP:** **`rescan`** block scan for wallet scripts (`wallet/scan.go`, `scanned_txs` in `wallet.json`; **`listtransactions`** / **`listsinceblock`**)
- [x] **MVP:** **Wallet restart persistence** - **Pebble** `wallet.db` (pure-Go LSM tx index + scan cursor) beside `wallet.json`; migrates legacy **`scanned_txs`**; live index on sequential **`ConnectBlock`** (`wallet/txdb`, `wallet/txindex.go`, `node/wire_wallet_live.go`)
- [x] **MVP:** **Wallet UTXO scan cache** - **`wallet_utxo_scan.cache.json`** persisted load/save for **`listunspent`** / fund paths (`store/wallet_utxo_cache.go`, `rpc/wallet_utxo_cache.go`)
- [x] **MVP:** **Chain connect checkpoint** - **`chain_active.manifest.json`** written with **`utxo.cache`** saves (`store/chain_active_manifest.go`, `node/utxo_snapshot_guard.go`)
- [x] **MVP:** **`importmulti`** `keys` array imports WIF cosigners (`extra_privkeys_hex`)
- [x] **MVP:** Full wallet RPC subset for operator use (UTXO-cache fast paths, PSBT utilities, HWI `signer_cmd`); deep Core tx-index edge cases are intentional differences where DogeGo uses `wallet.db` / txindex.
- [x] **MVP:** PSBT utility RPCs - **`decodepsbt`**, **`analyzepsbt`**, **`combinepsbt`**, **`joinpsbts`**, **`utxoupdatepsbt`**, **`finalizepsbt`**, **`createpsbt`**, **`converttopsbt`**
- [x] **MVP:** PSBT wallet RPCs - **`walletcreatefundedpsbt`**, **`walletprocesspsbt`**, **`descriptorprocesspsbt`**, **`psbtbumpfee`** (alias; built-in wallet; BIP32 deriv paths on fund/process)
- [x] **MVP:** **`dogego_importwalletdat`** - native BDB read (unencrypted **or encrypted** via `options.passphrase`; Core `CCrypter` SHA512+AES-256-CBC) + Core `dumpwallet` fallback (`core_rpc_addr`) or text dump import; native import returns **`keypool_hint`**, **`pool_indices_replayed`**, and pool metadata when Core keypool entries are present; runs **`keypoolrefill`** on HD wallets
- [x] **MVP:** **`dogego_probewalletdat`** - dry-run BDB probe (encrypted flag, key/watch/**pool** counts, **`pool_pubkeys`**, **`pool_entries`**, **`pool_keys_matched`**, **`pool_keys_unmatched`**, **`pool_index_min`/`pool_index_max`**, **`pool_indices_replayed`**, `encrypted_keys`, `needs_passphrase`, `can_import`); Receive tab + `/api/wallet/probe-walletdat`
- [x] **MVP:** **`dogego cert wallet-migration`** - cross-platform offline cert + on-disk probe/decrypt dry-run or **`-live-probe`** / **`-live-import`** RPC probe/import (`walletmigration/rpc_live.go`; `-require-wallet-dat`; mirrors `wallet_migration_cert.ps1`)
- [x] **MVP:** **Offline synthetic `wallet.dat` fixtures** - minimal BDB writer + unencrypted/encrypted/**descriptor-encrypted** E2E (`wallet/bdb/fixture.go`, `wallet/corewallet/fixture.go`; `TestExecImportWalletDatEncryptedDescriptorSyntheticFixture`; native extract emits Core `timestamp,wif` dump lines for `importwallet`)
- [x] **MVP:** **`scripts/provision_wallet_dat_fixture.ps1`** - locate Core `wallet.dat`, set `DOGEGO_WALLET_DAT`, live `dogego cert wallet-migration` probe for dogego-live runners
- [x] **MVP:** **weekly live wallet.dat import gate** - `dogego cert weekly` and `ci_scheduled_weekly_live.ps1` run RPC import when `DOGEGO_WALLET_DAT` / `-RequireWalletDat` / `DOGEGO_WALLET_DAT_REQUIRED=1`
- [x] **MVP:** External signer subset - **`signer_cmd`**, **`enumeratesigners`**, **`signerdisplayaddress`**, PSBT hook in **`walletprocesspsbt`**
- [x] **Operator UX:** **Settings external signer test** - `POST /api/signer-test` (HWI enumerate from form or saved `signer_cmd`; loopback only) - **S** (`ui/signer_probe.go`, Settings Advanced)
- [x] **Operator UX:** **Receive HD keypool refill** - `POST /api/wallet/keypool-refill` (Core `keypoolrefill`; optional `new_size`; Address book toolbar + `hd_wallet` on GET `/api/wallet`; after wallet.dat import with pool-only rows) - **S** (`ui/wallet_keypool.go`, Receive tab)
- [x] **Declined (out of scope):** Native USB/HID signer bridge without HWI subprocess - use HWI-compatible `signer_cmd` instead.

---

## Ongoing (all phases)

- Fuzzing / differential tests vs Core
- [x] **MVP:** [docs/INTENTIONAL_DIFFERENCES.md](docs/INTENTIONAL_DIFFERENCES.md) vs Core (storage, BIP152, witness, wallet)
- **Dogecoin protocol lock** - mainnet consensus follows Core (no protocol forks); audited via offline cert + differential harness + optional live Core compare
- Document intentional differences (keep updated as parity improves)
- **Documentation & integrator UX** - Phase 12 (full RPC cookbooks, embedded markdown in web UI, OpenAPI)
- Performance tuning

---

## References in this repo

| Area | Core paths (indicative) |
|------|-------------------------|
| P2P | `src/net.cpp`, `src/net_processing.cpp`, `src/protocol.*` |
| Consensus | `src/validation.cpp`, `src/dogecoin.cpp`, `src/pow.cpp` |
| Script | `src/script/` |
| Chain params | `src/chainparams.cpp`, `src/chainparamsbase.cpp` |
| RPC | `src/rpc/` |

When in doubt, **Core is the specification**.
