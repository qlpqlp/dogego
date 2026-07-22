# Standalone full-node acceptance matrix (Core-equivalent bar)

This matrix defines when DogeGo can be considered a practical standalone full Dogecoin node for operators. The bar is behaviorally aligned with Dogecoin Core on consensus-critical paths, operational recovery, and common RPC workflows.

Legend:
- **Status**: `done`, `partial`, `missing`
- **Pass criteria**: hard acceptance gate for each area
- **Test strategy**: how we prove the gate

## 1) Consensus and script parity

| Area | Current status | Pass criteria | Test strategy |
|---|---|---|---|
| Header consensus parity (`nBits`, retarget, MTP, checkpoints, auxpow boundaries) | partial | 100% pass on imported Core vector corpus + no false rewinds on healthy chains | Differential test harness: replay Core header sequences and compare accept/reject + tip selection |
| AuxPoW parent chain ID (Core `CAuxPow::check`) | done | Accept non-Dogecoin parent chain IDs; reject only parent encoding Dogecoin `0x62`; no false stall at ~510k headers | `consensus/headers_validate.go` (`checkAuxPow`); unit tests; post-aux watchdog + RPC fields; operator scripts (`check_header_progress.ps1`, `upgrade_post_aux_verify.ps1`, `sync_status.ps1`); mainnet datadir past 510k without header wipe (field evidence) |
| Block validity parity (`CheckBlock` + contextual checks) | partial | Identical reject reason class for invalid blocks in regression corpus | Corpus replay from Core invalid block tests, plus fuzzed malformed blocks |
| Script interpreter parity for legacy non-witness paths | partial | Identical script success/failure for full script test corpus | Import Core script JSON vectors and add Dogecoin-specific fixtures |
| Policy-vs-consensus separation | partial | Consensus accepts all valid blocks while policy rejects only mempool standardness | `consensus/policy_consensus_dual_path_test.go` (min relay + coinbase) |
| Reorg correctness under adversarial forks | partial | Deterministic best-chain choice by cumulative work under fork storms | `rejectIncomingForkIfPeerWorkHigher`, storm + mock peer tests (`TestEnsureIncomingForkWins_*`); live multi-peer reboottestnet soak backlog |

### Current implementation evidence (consensus differential track)

| Acceptance area | Evidence now in tree | Coverage status |
|---|---|---|
| Core difficulty retarget vectors | `consensus/core_differential_harness_test.go` + `consensus/testdata/core_difficulty_vectors.json` (`TestCoreDifficultyDifferentialVectors`) | partial |
| Core header accept/reject vectors | `consensus/core_header_differential_harness_test.go` + `consensus/testdata/core_header_vectors.json` (`TestCoreHeaderDifferentialVectors`; MTP via `TestCoreHeaderDifferentialSuiteIncludesMTP` → `TestValidateHeadersRejectsTimeRegression`) | partial |
| Core block filter vectors | `consensus/blockfilter_core_test.go` + `consensus/testdata/blockfilters_core.json` | partial |
| Core block validity vectors | `consensus/core_block_differential_harness_test.go` + `consensus/testdata/core_block_vectors.json` (`TestCoreBlockDifferentialVectors`; stored connect through **512** blocks via `chain_tip_height`) | partial |
| Mainnet field header PoW (operator datadir) | `consensus/core_mainnet_field_harness_test.go` (`TestCoreMainnetFieldHeaderPoW` on `dogedata/mainnet`); valid-PoW block hex + **stored connect 1-3** in `mainnet_field_blocks.json` (`TestCoreMainnetFieldStoredBlockConnect`); multi-tx **15504** (`consensus/testdata/mainnet_field_block_15504.hex`, `TestCoreMainnetFieldMultiTxBlock15504`); **`dogego cert field-evidence`** / `fieldevidence/suites.go` / `scripts/field_evidence_cert.ps1`; export via `scripts/export_mainnet_field_blocks.ps1` | partial |
| Core mempool policy vectors | `consensus/core_mempool_differential_harness_test.go` + `consensus/testdata/core_mempool_vectors.json` (**58** templates: coinbase, package limits, RBF incl. too-many-descendants/conflicts/new-unconfirmed-input, double-spend, maturity, BIP68 non-final, P2PKH/P2SH/bare-multisig/CLTV/CSV script-checked spends, PQ OP_RETURN + carrier P2SH relay, standardness/discourage-nop/datacarrier/p2sh-redeem rejects) | partial |
| Core script path vectors | `consensus/core_script_differential_harness_test.go` + `consensus/testdata/core_script_vectors.json` (100+ stack/template rows); Core `script_tests.json`: `TestCoreScriptTestsRunnerSubset` (**1059/1059** legacy; witness declined) | partial |
| `reindextx` operator maintenance | `rpc/reindextx_integration_test.go` | partial |
| Tx index repair from rawblocks | `store/txindex_repair_integration_test.go`, `store/txindex_sparse_test.go`, `store/txindex_sparse_through_zero_test.go` (`TestRepairTxIndexIfSparseAtHeightZero`), `node/txindex_funding_height_test.go`, `node/connect_txindex_repair_test.go` (proactive repair on connect funding/stall errors) | partial |
| `reindexblockfilters` operator maintenance | `rpc/reindexblockfilters_integration_test.go` | partial |
| Journal tail repair (1-79 byte torn writes) | `store/journal_tail_repair_test.go` | partial |
| Header segment crash recovery | `store/header_segments_test.go` (tail repair, checkpoint repair, stale temp purge, invalid/empty append rejection, genesis+remainder batch append path), `node/crash_header_segments_test.go`, `store/crash_active_write_test.go` | partial |
| Active-write kill simulation (Put + segment append) | `store/rawblock_subprocess_kill_test.go` (`TestSubprocessKillDuringRawPut`, `TestSubprocessKillDuringHeaderSegmentAppend` - stall after segment `.tmp` write via `atomicWriteFileStall`), `node/crash_header_segment_kill_sweep_test.go` | partial |
| Stateless testmempoolaccept Core parity | `consensus/testdata/mempool_parity_rpc.json`, `scripts/core_mempool_parity_probe.ps1`; **58** templates in `core_mempool_vectors.json` | partial |
| `truncatetoheight` RPC wiring | `rpc/truncatetoheight_test.go` | partial |
| `verifychain` level 4 + `gettxoutsetinfo` (offline) | `rpc/verifychain_level4_integration_test.go`, `rpc/gettxoutsetinfo_integration_test.go`; bounded RPC `SyncUtxo` during IBD (`SyncUtxoCacheBounded`); operator `syncutxo` RPC | partial |
| `testmempoolaccept` vs policy corpus (RPC path) | `rpc/testmempoolaccept_differential_integration_test.go` (**58/58** templates via `execTestMempoolAccept` / `AcceptMempoolTxPolicy`) | partial |
| Web wallet UTXO-cache API (solo miner) | `ui/wallet_fast_test.go` (`/api/wallet`, `/api/wallet/txs`, `/api/wallet/txs.csv`, `/api/wallet/utxos`); **`ui/wallet_rescan_test.go`** (`POST /api/wallet/rescan`, **`wallet_scan_index_ok`** on GET `/api/wallet`); **`ui/address_scan_wallet_test.go`**, **`ui/blockstep_api_test.go`** (`TestBlockStepAddressWalletFastHTTP` - BlockStep `/api/blockstep/address` wallet fast path); address book HTTP (`TestWalletAddressNewAPI`, `TestWalletAddressLabelAPI`); `rpc/wallet_getwalletinfo_fast_test.go` (wallet_index_height / needs_rescan / **`dogego_wallet_scan_index_ok`**), `rpc/wallet_ui_list_utxo_test.go` (`TestExecListTransactionsWalletManyUtxosLightPath`, **`TestExecListTransactionsWalletManyUtxosUsesScanIndex`**, **`TestExecListTransactionsSendFeeFromWalletDB`**, **`TestExecGetTransactionWalletHexAndFeeFromWalletDB`**, **`TestExecListSinceBlockSendFeeFromWalletDB`**), `rpc/wallet_rescan_test.go` (`TestRescanWalletSkipsSyncUtxoWhenCaughtUp`), `rpc/wallet_rpc_test.go` (`TestWalletRPCListUnspentMaximumCountAfterFiltering`); `node/connect_catchup_test.go`, `node/utxo_snapshot_guard_test.go` (caught-up connect lag + UTXO snapshot skip); **`node/wallet_catchup_test.go`** (startup catch-up rescan skip/incremental); `ui/core_wallet_probe_test.go` (`spendable_utxo_count`, address book keypool/core-pool index counts, rescan metadata, **`wallet_scan_index_ok`**, `setlabel` round-trip on `/api/core-wallet-probe`, `wallet_tx_hex_ok` / `wallet_tx_fee_ok`, PSBT + `keypool_topup_ok`, signer_cmd) | partial |
| Core wallet.dat migration (native BDB) | `wallet/bdb`, `wallet/corewallet` (`crypter_test.go`, `extract_test.go`, `TestFixtureWalletDatProbePoolCount`); **`wallet/pool_replay.go`** HD keypool replay (`TestExecImportWalletDatNativePoolIndicesReplayed`, `TestExecImportWalletDatMixedPoolHDReplay`); `dogego_probewalletdat` / `dogego_importwalletdat` RPC (**`pool_count`**, **`pool_pubkeys`**, **`pool_entries`** with `spends_key_matched`, **`pool_keys_matched`**, **`pool_keys_unmatched`** pool-only rows, **`pool_unmatched_entries`**, **`pool_unmatched_hint`**, **`keypool_refill_size`**, **`keypool_hint`**, **`pool_index_min`/`pool_index_max`**, **`pool_indices_replayed`**, **`pool_core_indices_stored`**); `/api/wallet/probe-walletdat` + `/api/wallet/import` with `passphrase`; **`GET /api/core-wallet-probe`** when `DOGEGO_WALLET_DAT` set; **`dogego cert wallet-migration`** / `walletmigration/verify.go` / `scripts/wallet_migration_cert.{ps1,sh}` | partial |
| Cross-platform offline certification | **`dogego cert offline`** (`offlinegate/`; `scripts/ci_offline_gate.{ps1,sh}`); prerequisite bundle `scripts/cert_offline_prerequisites.{ps1,sh}`; drift guards in `docs/scripts_cert_test.go` | partial |
| Raw block `.bin.tmp` crash leftovers | `store/rawblock_tmp_purge_test.go`, `node/crash_rawblock_midwrite_test.go` | partial |
| Block filter `.dat.tmp` crash leftovers | `store/blockfilter_tmp_purge_test.go`, `node/crash_blockfilter_midwrite_test.go`, `TestSubprocessKillDuringBlockFilterPut` | partial |
| Tx index atomic write + `.tmp` purge | `store/txindex.go`, `store/txindex_tmp_purge.go`, `TestSubprocessKillDuringTxIndexWrite`, `node/crash_index_filter_sweep_test.go` | partial |
| Operator certification script | `scripts/operator_workflow_cert.{ps1,sh}`, **`dogego cert operator`** (`operatorworkflow/verify.go`), **`dogego cert offline`**, **`dogego cert field-evidence`**, **`dogego cert wallet-import`** (`walletimport/verify.go`; `scripts/wallet_import_cert.{ps1,sh}`), **`dogego cert wallet-migration`** (`scripts/wallet_migration_cert.{ps1,sh}`), **`dogego cert pq`** (`pqcert/suites.go`, `scripts/pq_cert.{ps1,sh}`), **`scripts/cert_offline_prerequisites.{ps1,sh}`** (ROADMAP prerequisite bundle), `scripts/ibd_soak_cert.ps1`, `scripts/core_operator_workflow_cert.ps1`, `scripts/core_compare_with_core.ps1`, `scripts/core_end_to_end_workflow.ps1`, `scripts/core_maintenance_workflow.ps1`, `scripts/core_restart_resume_check.ps1`, `scripts/core_wallet_workflow.ps1`, `scripts/wallet_import_cert.ps1`, `scripts/wallet_migration_cert.ps1`, `scripts/field_evidence_cert.ps1`, `scripts/corruption_soak_cert.ps1`, `scripts/corruption_inject_live.ps1`, `scripts/corruption_inject_soak.ps1`, `scripts/extended_operator_soak.ps1`, `scripts/export_mainnet_field_blocks.ps1`, `scripts/ibd_convergence_check.ps1`, `scripts/upgrade_post_aux_verify.ps1`, `scripts/core_parity_probe.ps1`, `scripts/node_health.ps1`, `scripts/restart_node.ps1`, `scripts/log_ibd_progress.ps1`, `scripts/watch_sync.ps1` | partial |
| Full Core `script_tests.json` interpreter corpus | `consensus/script_asm.go`, `consensus/script_eval.go`, `VerifyScriptTestSpend`; `TestCoreSighashDifferentialHarness` (Core `sighash.json`); `TestCoreScriptTestsRunnerSubset` (**1059/1059** legacy rows; witness rows **declined** per segwit-disabled policy); `TestCoreScriptTestsWitnessRowsIntentionallyDeclined` | partial (witness declined) |
| Production script path for non-template `scriptPubKey` | `verifyInputEval` fallback in `VerifyScript` (`script_eval_verify.go`, `TestVerifyScriptEvalSimpleTrue`) | partial |
| UTXO `hash_serialized` linear vs rebuild | `store/utxo_hash_differential_test.go` (`TestUtxoSerializedHashLinearVsRebuild`) | partial |
| UTXO rewind + replay fidelity | `store/utxo_hash_differential_test.go` (`TestUtxoSerializedHashReorgReplay`) | partial |
| Sequence-lock prev height when txindex lags | `consensus/prevout_height_test.go`, `store/utxo_height_lookup_test.go`; `node/datadir_connect_diag_test.go` (`-tags datadir_diag`); `node/connect_txindex_repair.go` (urgent sparse repair on connect funding/stall errors, retry connect after repair) | partial |

## 2) Chainstate, undo, and crash consistency

| Area | Current status | Pass criteria | Test strategy |
|---|---|---|---|
| UTXO correctness across long IBD + reorgs | partial | Deterministic UTXO hash snapshots match expected after every checkpoint window | Snapshot-hash tests every N heights + cross-check tool against known-good nodes |
| Undo/replay fidelity for disconnect/connect cycles | partial | Reorg/disconnect replay yields identical chainstate as linear sync | `store/utxo_reorg_stress_test.go`, `node/chain_reorg_utxo_stress_test.go` (replay via `RebuildFromChain` / truncate; no Core undo files yet) |
| Crash consistency (kill -9 / power-loss model) | partial | No chain corruption; automatic recovery converges without manual steps | Fault injection: random process kill during write-heavy phases + startup repair validation |
| Truncate + UTXO replay after header rewind | partial | `hash_serialized` at truncated tip matches `RebuildFromChain` reference | `node/chain_truncate_utxo_test.go` |
| Raw block stub purge after crash | partial | Undersized `rawblocks/*.bin` removed; node continues without manual purge | `node/crash_rawblock_fault_test.go` |
| Headers mid-write tail repair | partial | Torn monolith tail repaired on reopen; segment tail repair + `.tmp` purge; `headers_aux.bin` torn tail truncate on open; `autoRecoverSweep` continues | `node/crash_headers_midwrite_test.go`, `node/crash_header_segments_test.go`, `store/header_segments_test.go`, `store/header_aux_recover_test.go` |
| Header segment checkpoint realign | partial | Manifest ahead of `headers_sync.json` truncates on open | `store/header_segments_test.go` (`TestHeaderSegmentCheckpointRepair`) |
| Operator workflow (no P2P) | partial | IBD-free path: bodies, recovery sweep, truncate, UTXO hash | `node/operator_workflow_cert_test.go` |
| Prune semantics correctness | partial | Pruned nodes keep consensus correctness and expected RPC behavior limits | `store/prune_raw_test.go`; RPC prune integration + `TestExecVerifyChainAfterPrune` (verifychain level 2 + getblock tip after prune) |

## 3) P2P network behavior and robustness

| Area | Current status | Pass criteria | Test strategy |
|---|---|---|---|
| Peer management parity (addrman quality, rotation, scoring) | partial | Stable sync under mixed-good/bad peer populations; no persistent deadlock/stall | Offline: inbound eviction + `TestEclipseInboundPressureSoak` (protect addnode/HB, /16 preference). Live multi-peer eclipse soak still open; `block_peer_score_persist_test.go` archival merge |
| Message coverage parity (required protocol paths) | partial | Required message flows validated against Core-shaped behavior | Protocol conformance tests for `headers`, `getdata`, `inv`, `getheaders`, reject paths |
| Stall handling and timeouts | partial | Sync always makes forward progress or rotates peers automatically | **Core-style body IBD pump** (1.5s proactive getdata on primary/relay/assist; `ensureBodyDownloadArmed` clears `idleFull` deadlock); 90s body-only stall recovery + assist relaunch; long-haul soak tests with throttled and flaky peers; `ibd_stall_genesis_test.go`, `ibd_body_pump_test.go` |
| Forward block IBD (ancient heights, contiguous frontier) | partial | `contiguous_raw_height` advances past genesis; early coinbase-only blocks (~190-250 B, e.g. height **10006** = **213 B**) count as stored when ≥140 B on mainnet `< 500k`; batch fetch does not deadlock on header lock; connect catch-up closes chainActive lag after restart; sequence-lock prev heights fall back to UTXO when txindex lags; invalid undersized peers rotated at any height | `store/rawbody_adequate_test.go`; `node/block_stub_peer_test.go` (`TestMinRawBlockBytesAllowsReal213ByteBlockAtHeight10006`); `node/connect_catchup_test.go`; `node/block_stub_peer_test.go`; `node/block1_size_check_test.go`; `consensus/prevout_height_test.go`; `node/datadir_connect_diag_test.go` (`-tags datadir_diag`); operator: `ibd_convergence_check.ps1`, `log_ibd_progress.ps1`, `node_health.ps1` |
| Inbound serving correctness | partial | Inbound `getdata/getheaders` responses are complete and policy-correct | `TestInboundServeBlackBox`, `TestInboundServeBlackBoxConfirmedTxAndNotFound`, `getheaders_serve_test.go`, `getdata_serve_test.go` |
| BIP152 compact block relay (v1) | partial | `sendcmpct` HB negotiate; `cmpctblock` announce/reconstruct/getblocktxn; AuxPoW → full `inv`/`block`; offline AuxPoW/cmpct edges + `dogego cert bip152-soak`; live HB soak operator-owned | `wire/cmpctblock_test.go`, `wire/cmpct_auxpow_test.go`, `node/cmpct_test.go`, `node/getdata_serve_test.go`, `ui/core_bip152_probe_test.go`, `runner/bip152_soak.go` |

## 4) Mempool and relay policy parity

| Area | Current status | Pass criteria | Test strategy |
|---|---|---|---|
| Mempool admission parity (standardness, fee gates, package limits) | partial | Policy results align with Core for shared policy set | `consensus/core_mempool_differential_harness_test.go` + `TestCoreMempoolVectorTemplatesCovered` (**58** templates); `TestCoreDifferentialCorpusGate`; RPC `testmempoolaccept` for all **58** in `rpc/testmempoolaccept_differential_integration_test.go` |
| Ancestor/descendant accounting parity | partial | No package acceptance/eviction divergence in tested envelope | Diamond/fan-out + seeded DAG property tests (`mempool/package_dag_property_test.go`); live package-RBF soak still open |
| Fee estimation quality and persistence | partial | Stable estimates within tolerance during workload replay | Historical replay harness + restart persistence checks |
| RBF and replacement behavior | partial | Replacement acceptance/rejection matches expected rule set | Directed tests: insufficient/sufficient fee, not-replaceable, FullRBF, **too-many-descendants**, **PaysForRBF descendant conflict package** (child underpay reject + ancestor not charged); live package-RBF soak still open |

## 5) RPC/operator compatibility

| Area | Current status | Pass criteria | Test strategy |
|---|---|---|---|
| Core workflow RPC compatibility (chain, mempool, mining, wallet basics) | partial | High-value workflow suite passes unchanged scripts | End-to-end operator scripts against DogeGo and Core side-by-side; web UI `GET /api/core-operator-cert` (**17** live web gates incl. Milestone D **setup-parity**, `runner_readiness`, **BIP152 HB**, **mining GBT/aux**, **PQ format**, **IBD convergence snapshot**, **addrman snapshot** via `GET /api/core-addrman-probe`) + `GET /api/core-status` (cached cert + mempool corpus/parity) + `GET /api/core-end-to-end-probe` (`offline_corpus`, `bip125_offline`, `mempool_parity`, `mining`) + `GET /api/core-probes` + `GET /api/core-mining-probe` + `GET /api/core-runner-probes`; `/api/summary` exposes `dogego_operator_cert_*`, `dogego_mempool_*`, and `dogego_utxo_body_replay_*` when applicable; startup probe-cache warm (~8s) |
| dogego-live scheduled CI (reboottestnet) | partial | Weekly bundle + optional Milestone B soak green on self-hosted runner | **`dogego cert weekly-live`**, **`dogego cert live-soak`**, **`dogego cert setup-parity`**, **`dogego cert provision -run-setup`**; mirrors `ci_scheduled_weekly_live.ps1` / `ci_milestone_b_full_gate.ps1`; `-skip-scripts` preflight smoke; see [CORE_SIDE_BY_SIDE_WORKFLOWS.md](CORE_SIDE_BY_SIDE_WORKFLOWS.md) workflow 10 |
| JSON-RPC early listen + warmup (-28) during IBD startup | done | Port binds with Web UI; `-28` until dispatch wired; full RPC dispatch before startup UTXO catch-up (not blocked by SyncUtxoCache) | `rpc/early_server.go`, RPC wiring before async startup in `node/run.go` |
| Single instance per network datadir (Core `.lock` analogue) | done | Second `dogego node` on same `<datadir>/<network>/` fails fast with clear error | `store/process_lock.go`; wired in `node/run.go`; `TestProcessLock*` |
| Error-code and message compatibility for common failures | partial | Stable code class parity for top operator RPCs | `rpc/operator_rpc_errors_test.go` (pruneblockchain, truncatetoheight, reindextx, recoverheaders, getblock, sendrawtransaction classes) |
| Verify/reindex/recovery operational safety | partial | Recovery RPCs are idempotent and safe during runtime | `TestExecReindexTxIdempotentSecondPass`, `TestExecReindexBlockFiltersIdempotentSecondPass`, `TestExecDogegoRecoverHeadersIdempotentRestart`, `TestExecTruncateToHeightIdempotentSecondCall`; extended load soak backlog |

## 6) Automatic recovery guarantees (no manual button required)

| Area | Current status | Pass criteria | Test strategy |
|---|---|---|---|
| Startup auto-heal for headers/raw/index/filter inconsistencies | partial | Node self-recovers at boot and resumes sync without manual intervention | Corrupt-on-disk fixtures + startup validation suite |
| Runtime periodic self-heal and rewind-safe re-entry | partial | Live node repairs and resumes without operator action or stuck loops | Soak tests with timed corruption injection |
| Recovery boundedness and safety | partial | No data-loss beyond required rewind window; no endless oscillation | Invariant tests for max rewind depth and retry backoff |

## Current implementation evidence (recovery track)

This section maps matrix goals to concrete tests already in-tree so progress is auditable.

| Acceptance area | Evidence now in tree | Coverage status |
|---|---|---|
| Startup auto-heal: genesis/header sanity guards | `node/auto_recovery_test.go` (`TestAutoRecoverGenesisSanityAcceptsMatchingGenesis`, `TestAutoRecoverGenesisSanityRejectsMismatchedGenesis`); `TestAutoRecoverSweepEnsuresLocalGenesis`; `TestAutoRecoverSweepReconcilesRawBlockSyncCheckpoint` | partial |
| IBD stall recovery (genesis-aware) | `node/ibd_stall.go`, `node/ibd_stall_genesis_test.go` (`TestIbdStallRecoverIntervalGenesis`, `TestIbdStallRecoverIntervalConnectCaughtUp`); mid-depth body IBD stall window **10 min** when stored through ≥ 1000; **90s** when connect caught up (body-only phase); **Core-style body pump** (`node/ibd_body_pump.go`, `ibd_body_pump_test.go`) | partial |
| Connect catch-up during IBD (chainActive lags stored bodies) | `node/connect_catchup.go`, `node/connect_catchup_worker.go`, `node/utxo_connect_lock.go`, `node/blockfilter_contiguous_ibd.go`, `node/blockfilter_catchup_worker.go`, `node/utxo_ibd.go`, `node/chain_rpc_early.go`, `node/ibd_progress_snapshot.go`; filter repair capped to contiguous bodies (`store/blockfilter_repair.go`); async `saveutxosnapshot` + non-blocking `SaveSnapshot` (`store/utxo_snapshot.go`, `store/utxo_snapshot_async.go`); tests: `connect_catchup_test.go`, `connect_catchup_ibd_cap_test.go`, `ibd_progress_snapshot_test.go`, `utxo_connect_lock_test.go`, `blockfilter_contiguous_ibd_test.go`, `blockfilter_catchup_worker_test.go`, `utxo_snapshot_async_test.go`; RPC `dogego_connect_blocks_per_minute`, `dogego_connect_catch_up_{passes,batch,interval_ms}`, `dogego_utxo_snapshot_*`, `dogego_utxo_connect_in_flight`, `dogego_syncutxo_in_flight`; operator `node_health.ps1`, `nudge_connect.ps1`, `save_utxo_snapshot.ps1`, `ibd_convergence_check.ps1`, `restart_node.ps1` | partial |
| Startup/runtime sweep safety and continuation | `node/auto_recovery_test.go` (sweep/mini-soak/corrupt index, `TestAutoRecoverSweepRefreshesContiguousTipFromDisk`), `node/startup_recovery_convergence_test.go`, `node/crash_header_aux_test.go` (headers_aux torn tail) | partial |
| Post-rewind re-entry correctness | `node/auto_recovery_test.go` (`TestAutoRecoverPostRewindResetsSyncState`) | partial |
| Stuck ancient header tip + no bodies (auto genesis reset) | `node/headers_stuck_ancient.go` (`MaybeResetStuckAncientHeaderChain`, `maybeResetStuckAncientInSweep` in `autoRecoverSweep`, header-advance watchdog) | partial |
| Recoverable rewind classes: bad nBits/checkpoint/aux validation | `node/auto_recovery_test.go` (`TestRunLocalHeaderJournalRecoveryRewindsOnBadNBits`, `TestRunLocalHeaderJournalRecoveryRewindsOnCheckpointMismatch`, `TestRunLocalHeaderJournalRecoveryRewindsOnAuxpowValidationErr`) | partial |
| Recovery boundedness / no oscillation (local fixtures) | `node/headers_rewind_retry_test.go` (`TestMaybeRewindOnBadNBitsResetsRepeatStateAfterGenesisReset`, `TestMaybeRewindOnBadNBitsEachRewindMovesTipBack`, `TestBadNBitsRecoveryDecision`) + `node/auto_recovery_test.go` (`TestAutoRecoverSweepIsIdempotentOnCorruptionFixtures`) | partial |
| Non-recoverable corruption safety (no unsafe rewind) | `node/auto_recovery_test.go` (`TestAutoRecoverHeadersReturnsErrorOnCorruptJournal`) | partial |

### Remaining high-priority gaps for automatic recovery

- Crash/power-loss fault-injection suite across `headers/`, `rawblocks/`, `indexes/tx`, and `filters/basic` (subprocess kill: raw Put, header segment, filter, tx index; `node/crash_index_filter_sweep_test.go`).
- Extended runtime soak (beyond deterministic unit-test mini-soak) with timed corruption injection and convergence assertions (`scripts/ibd_monitor.ps1`, `scripts/ibd_convergence_check.ps1`, `scripts/log_ibd_progress.ps1`, `DOGEGO_IBD_CONVERGE=1`).
- ~~Body download stall when primary session paused and block-assist latch stuck~~ - mitigated: `reconcileHeaderCatchUpPending`, `seedBlockAssistCandidates`, `maybeEnsureBlockAssistDuringNoPrimary`, `initProgressiveRawAtStartup` (`node/ibd_header_body_priority.go`, `block_assist_seed.go`, `ibd_stall.go`; tests in `ibd_header_body_priority_test.go`, `block_assist_seed_test.go`).
- ~~BIP158 filter catch-up visibility~~ - done: `dogego_filter_index_through`, `dogego_filter_index_lag` on `getblockchaininfo` (`rpc/blockfilter_diag.go`, `node/filter_index_progress.go`).
- Bounded-retry/backoff invariants proving no endless rewind oscillation loops - `badNBitsRecoveryDecision` + `TestBadNBitsRecoveryDecision` (genesis reset below 500k; peer rotation at mainnet scale).

## Exit gate: "Standalone-ready"

DogeGo is considered standalone-ready only when all conditions are met:

0. **Protocol lock:** mainnet consensus rules match Dogecoin Core (no protocol forks); differential harness and live Core compare show no unintended consensus divergence.
1. Every matrix row is `done`.
2. Differential harness shows no consensus divergence in accepted/rejected headers/blocks for the adopted corpus.
3. Crash-consistency and corruption-recovery suites pass repeatedly on CI and a long-haul soak run.
4. Main operator workflows (sync, query, recovery, restart, pruning, wallet basics) pass without manual repair steps.
