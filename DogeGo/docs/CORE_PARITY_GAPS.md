# Core parity gaps (Dogecoin Core → DogeGo)

Dogecoin **Core** (`src/`) is the consensus and operator specification. DogeGo targets **Core-compatible behavior** on critical paths but is **not production-certified** until the standalone acceptance matrix is complete.

**Protocol lock:** DogeGo does not change mainnet consensus rules (no protocol forks). Rows below are **implementation parity** gaps, not alternate Dogecoin rules. Chain "fork" in tests means **reorg**, not a new activation.

**How to read status:**

| Status | Meaning |
|--------|---------|
| **partial** | MVP or scaffold exists; differential tests / soak / operator scripts not fully green |
| **open** | Large backlog item (often **XL** effort) |
| **declined** | Intentionally out of scope - see [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md) |

**Sources:** [ROADMAP.md](../ROADMAP.md) (unchecked `[ ]` lines), [STANDALONE_FULLNODE_ACCEPTANCE.md](STANDALONE_FULLNODE_ACCEPTANCE.md), web **Features** tab (`GET /api/capabilities` → `core_parity_gaps`).

---

## Exit gate (standalone-ready)

All of the following must be true ([STANDALONE_FULLNODE_ACCEPTANCE.md](STANDALONE_FULLNODE_ACCEPTANCE.md) § Exit gate):

1. Every acceptance-matrix row is **done** (today: all **partial**).
2. Differential harness shows **no consensus divergence** on the adopted corpus.
3. Crash-consistency and corruption-recovery suites pass on CI **and** long-haul soak.
4. Main operator workflows pass **without manual repair** (IBD, restart, reindex, prune, wallet basics).

---

## Certification milestones (ROADMAP immediate track)

| Milestone | Gap | Effort |
|-----------|-----|--------|
| **A (field)** | Mainnet field header PoW + block hex connect; connect corpus **0-3** + sparse **100/200/272/10006** + multi-tx **15504** CheckBlock (committed `consensus/testdata/mainnet_field_block_15504.hex`); bundled store harness; checkpoint header accept; committed **auxpow** at **371337+** (`mainnet_field_auxpow.json`); **`dogego cert field-evidence`** (`fieldevidence/suites.go`) / `field_evidence_cert.ps1`; **`GET /api/core-field-evidence-probe`** + `field_evidence_live_cert.ps1`; live `TestCoreMainnetFieldHeaderPoW` when datadir synced | M - partial |
| **B (full)** | Extended runtime soak + timed corruption on headers/raw/index/filter; kill-and-restart convergence | L - see [MILESTONE_B_CERT.md](MILESTONE_B_CERT.md); **`dogego cert milestones-bde`** (offline code gates); `milestone_b_cert.ps1` tiers + `ci_milestone_b_full_gate.ps1`; cross-platform **`dogego cert live-soak`** (workflow 10) |
| **D (full)** | Stateful **24/24** Core compare green in scheduled CI via **`dogego cert weekly-live`** on `dogego-live` runner (workflow 10); offline corpus via **`dogego cert milestones-bde`** | M - partial |
| **E (full)** | **17** live web operator-cert gates (incl. mining GBT/aux + PQ format probe) + full workflow 10 sign-off; offline operator gates via **`dogego cert milestones-bde`** (`dogego cert enable-weekly` → provision → weekly-live → live-soak) | L - partial |

Partial scaffolds already exist (`consensus/core_*_harness_test.go`, `scripts/core_*_workflow.ps1`, `scripts/corruption_*`).

---

## Consensus & script

| Area | Status | Pass bar |
|------|--------|----------|
| Header consensus (nBits, retarget, MTP, checkpoints, auxpow) | partial | Six-hundred-header stored + forty-eight-header batch; batch bad-prev index 6+; bad-nBits index 24+ |
| Block validity (`CheckBlock` + contextual) | partial | Stored genesis-**512**-block connect (`chain_tip_height`) + CheckTransaction reject corpus |
| Script interpreter (legacy non-witness) | partial | **1059/1059** legacy `script_tests.json` rows; witness rows skipped (segwit disabled) |
| Policy vs consensus separation | partial | Valid blocks connect; policy rejects mempool-only |
| **Mainnet protocol lock** (no consensus forks) | partial | ROADMAP policy; offline differential + field evidence; live Core compare on `dogego-live` pending |
| Reorg under adversarial forks | partial | Best chain by work under fork storms (reorg, not protocol fork) |

Evidence: `consensus/testdata/core_*.json`, `TestCoreScriptTestsRunnerSubset` (**1059/1059** legacy rows; witness skipped).

---

## P2P networking

| Area | Status | Notes |
|------|--------|-------|
| **Core bucketed addrman** | **partial** | 256/1024 hash buckets + **64-deep per-bucket slot indices** (`TriedSlot` / `NewRefs`) + multi-ref new (up to **8**, Core `ADDRMAN_NEW_BUCKETS_PER_ADDRESS`) + Core-scale flat caps (**16384** tried / **65536** new); nKey + `learned_addrs.json` v3; occupancy maps for slot place/evict; inbound eviction-when-full (`AttemptEvictInboundForNew`) |
| Peer management under churn / eclipse pressure | partial | Inbound eviction-when-full: protect addnode (`addnodeMatchesSession`) + BIP152 HB; prefer crowded `/16` victims; offline soak `TestEclipseInboundPressureSoak` + eviction table tests. Live multi-peer eclipse soak still open. |
| Message coverage / protocol conformance | partial | Required flows vs Core-shaped behavior |
| Stall handling / forward IBD | partial | **Core-style body IBD pump** (1.5s proactive getdata on primary + relay; `ensureBodyDownloadArmed`; 90s body-only stall recovery + assist relaunch); body-IBD header pause; connect boost up to **512×8** passes when lag>8k (`connect_catchup_test.go`); RPC **`dogego_connect_catch_up_{passes,batch,interval_ms}`** + Web UI sync dock / Overview / restart-resume card; operator cert **restart_connect** row notes boost; **`GET /api/core-ibd-convergence-probe`** snapshot (**15th** live web gate); IBD CSV log column **`connect_boost`**; operator **`node_health.ps1`**, **`sync_status.ps1`**, **`watch_sync.ps1`**, **`log_ibd_progress.ps1`**, **`ibd_convergence_check.ps1`**, **`core_end_to_end_workflow.ps1`**; post-restart startup burst (32 rounds); **pre-P2P RPC + connect workers** (defer raw purge / autoRecoverSweep); extended soak open |
| Inbound serving (getdata/getheaders) | partial | Adversarial + reorg-policy tests: batch caps, witness/filtered inv, invalid header reject, getheaders 2000 cap, BIP157 filter range, duplicate getdata | `node/getdata_serve_test.go`, `node/getheaders_serve_test.go`, `node/filter_serve_test.go`, `node/headers_apply_test.go` |
| **BIP152 compact blocks** | **partial** | v1 HB relay **code-complete** + offline AuxPoW/cmpct edge tests; `dogego_cmpct_reconstruct_fallback_getdata` in probe schema; **`dogego cert bip152-soak`** (offline default; live PS1 optional). Extended mainnet/reboottestnet HB soak with advancing `dogego_cmpct_*` remains **operator-owned** (`scripts/bip152_live_soak_gate.ps1`, `DOGEGO_BIP152_LIVE_SOAK=1`). |
| **AuxPoW parent chain store** | **declined (matches Core)** | Neither node syncs Litecoin; parent header is embedded in each block's CAuxPow and validated by `checkAuxPow` / `CAuxPow::check` |

---

## Storage & chainstate

DogeGo **never** reads/writes Core `blocks/` + `chainstate/` LevelDB ([ROADMAP Phase 5](../ROADMAP.md)).

| Area | Status | Notes |
|------|--------|-------|
| UTXO long IBD + reorgs | partial | `hash_serialized` snapshot windows |
| Undo / disconnect replay | partial | `RebuildFromChain`; no Core `.undo` files |
| Crash consistency (kill -9 model) | partial | Subprocess kill tests partial; **utxo.cache** torn `.tmp` purge + corrupt quarantine on startup (`store/utxo_snapshot_crash_test.go`, `node/utxo_snapshot_crash_recovery_test.go`); full multi-hour soak open |
| Truncate + UTXO replay | partial | `chain_truncate_utxo_test.go` |
| Prune semantics | partial | pruneblockchain + getblockchaininfo integration | `rpc/pruneblockchain_integration_test.go` |
| Auto-heal at startup / runtime | partial | `auto_recovery_test.go`; bundled hash-mismatch locator purge at startup; `rawblocks_sync.json` stale probe/contiguous clamp in `autoRecoverSweep`; extended inject soak open |

---

## Mempool & relay

| Area | Status |
|------|--------|
| Admission (standardness, fees, packages) | partial - **58** templates; **`submitpackage` CPFP** package-unit min relay + `fees.effective-includes` + `replaced-transactions`; Features **`stateful_live`** embeds **`setup_parity_*`** from **`GET /api/core-setup-parity`** on reboot testnet + **`GET /api/mempool/stateful-status`**; live 24/24 on reboottestnet via scripts |
| Ancestor/descendant accounting | partial - diamond/fan-out + seeded DAG property suite (`mempool/package_dag_property_test.go`); live package-RBF vs Core still open |
| Fee estimation quality + persistence | partial - `fee_history.json` + `fee_estimates.dat` analogue |
| RBF replacement edge cases | partial - insufficient/sufficient fee, not-replaceable, **mempoolfullrbf**, **PaysForRBF conflict-set fees use descendant package** (`ConflictPackageFeeSize` → `DescendantFeesKoinu`/`DescendantSize`; underpay child + ignore-ancestor parent tests); descendant limit; **BIP125 rule 5** (max 100 replaced txs incl. descendants) + **rule 2** (no new unconfirmed inputs) in `mempool_rbf.go`; corpus rows **`rbf_too_many_conflicts`** / **`rbf_new_unconfirmed_input`**; live package-RBF vs Core still open |

---

## RPC, wallet, mining

| Area | Status |
|------|--------|
| Core workflow RPC (chain, mempool, mining, wallet basics) | partial - side-by-side scripts partial; **`core_wallet_workflow.ps1`** / **`GET /api/core-wallet-probe`** include optional PSBT round-trip (`walletcreatefundedpsbt` + `walletprocesspsbt`) and **`wallet_pq_send_ok`** when **`pq_commitments`** enabled |
| **dogego-live scheduled CI** | **partial** - **M** | Cross-platform cert chain: **`dogego cert workflow10`**, **`dogego cert provision`**, **`dogego cert weekly`**, **`dogego cert enable-weekly`**, **`dogego cert weekly-live`**, **`dogego cert live-soak`**, **`dogego cert setup-parity`**; mirrors `ci_scheduled_weekly_live.ps1` / `ci_milestone_b_full_gate.ps1`; `-skip-scripts` preflight smoke; Web UI **`GET /api/core-runner-probes`** + **`GET /api/core-workflow10-probe`**; see [CORE_SIDE_BY_SIDE_WORKFLOWS.md](CORE_SIDE_BY_SIDE_WORKFLOWS.md) workflow 10 |
| Web wallet API (large UTXO / solo mining) | partial - **`GET /api/wallet/txs`** + **`/api/wallet/txs.csv`** prefer **`WalletTxs.ListPage`** (`listtransactions` RPC bridge) before UTXO/scan fallbacks; wallet queries use **`FilterRowsByScriptSet`** / **`ForEachRow`** on the UTXO cache (no full-chain `DumpRows` sort on send, history, or BlockStep balance); BlockStep **`/api/blockstep/address`** uses wallet fast path (UTXO + **`wallet.db`**) for owned addresses and skips raw block walk when UTXO hits cap; **`GET /api/wallet/txs?type=sent`** and **`type=quantum`** read **`wallet.db`** scan index + **`LookupTxHex`** with PQ enrichment (`ui/wallet_pq_enrich.go`); internal self-transfer send amounts via **`SendDisplayKoinu`**; Settings → **Wallet** tab for **`nowallet`** / **`pq_commitments`**; **`pq_commitments` defaults on** when omitted from `wallet.json`; **`type=all`** uses **`wallet.db`** scan history when receive rows indexed (else UTXO receives + scan sends); sent-row **`hex`** lazy-loads from tx index or block height when missing from **`wallet.db`**; `/api/wallet` and **`GET /api/summary`** expose **`keypool_size`**, **`pool_core_indices_stored`**, **`hd_keypool_core_index`**, **`wallet_index_height`**, **`needs_rescan`**, **`wallet_scan_index_ok`**, **`wallet_history_fast_path`**, **`scanning`**, **`signer_cmd_configured`** from disk/RPC; **`POST /api/wallet/rescan`** (incremental or `{full:true}`) backfills **`fee_koinu`** + tx hex in **`wallet.db`**; wallet catch-up / **`rescan`** skip **`SyncUtxo`** when chainActive already covers contiguous bodies; `/api/wallet/utxos` use in-memory UTXO cache + **SpendScripts** matching when wired (`ui/wallet_fast.go`; `TestWalletUtxosHTTPAllSpendScripts`); `listunspent` uses the UTXO cache and applies `minimumAmount` / `maximumCount` after filtering (`TestWalletRPCListUnspentMaximumCountAfterFiltering`); **`listtransactions` / `gettransaction` / `listsinceblock` / `liststucktransactions` / `resendwallettransactions`** share the UTXO-cache light path + 20s row cache (`walletUIRowsCached`; skips UTXO receive walk when **`wallet.db`** has receive rows - **`TestExecListTransactionsWalletManyUtxosUsesScanIndex`**; **`dogego_wallet_listtransactions_utxo_walk`** on **`getwalletinfo`** when no receive rows; **`dogego_wallet_listtransactions_scan_pending`** while rescan runs before first receive rows; History tab defers heavy **`listtransactions`** during scan build (>64 UTXOs) and reloads when index ready); confirmed send **`fee`** + **`hex`** persisted in **`wallet.db`** at broadcast / block scan (`fee_koinu`, `WalletTxHexLookup`); dashboard auto-starts incremental rescan when caught up and (**`needs_rescan`** or **`wallet_listtransactions_utxo_walk`** with >64 UTXOs); Send fee hints on `/api/wallet/send`; **Receive address book** (`GET /api/wallet/addresses`, `GET /api/wallet/labels`, `POST /api/wallet/address/new`, `POST /api/wallet/address/label`; path/type sort in `wallet/address_sort.go`); Core-shaped `getaddressesbylabel` object; Features wallet probe exposes `spendable_utxo_count`, **`pq_commitments_ok`**, `keypoolsize`, `pool_core_indices_stored`, **`wallet_index_height` / `needs_rescan` / `wallet_scan_index_ok` / `wallet_history_fast_path` / `wallet_history_defer_reason` / `dogego_wallet_history_defer_reason` / rescan notes**, **`keypool_topup_ok`**, **`wallet_tx_hex_ok` / `wallet_tx_fee_ok`**, **`wallet_listtransactions_ok`** (<3s for 40 rows when not deferred), **`psbt_roundtrip_ok` / `psbt_bip32_deriv_ok`**, `label_roundtrip_ok`, `label_list_ok`, `enumeratesigners_ok`, **`signer_cmd_configured`**; non-wallet / pre-index history hex still partial |
| Solo testnet background miner | partial - `mine=true` includes mempool txs in each block (`generate_mempool_test.go`) |
| Error-code compatibility (top operator RPCs) | partial | Golden tests (**1568** subtests; `TestOperatorRPCGoldenSubtestCount`) incl. `dogego_importwalletdat` / `dogego_probewalletdat` arity paths + batch-72 aux/multisig/account paths + batch-71 chain-control/wallet-list paths + batch-70 psbt/mining paths + prior batches; `operator_rpc_errors_test.go` |
| Web wallet Send fee hints | partial | `/api/wallet/send` returns `fee_hint` on insufficient funds (`wallet_send_hints.go`; `TestWalletSendHTTPInsufficientFundsFeeHint`) |
| Recovery RPC idempotency under load | partial | Double-call reindextx / reindexblockfilters / recoverheaders / truncatetoheight / savemempool / loadmempool / upgradetxindex / **pruneblockchain** in offline CI gate (`ci_offline_gate.*`, `dogego cert offline`; aligned suites incl. autostart/founder/runner + wallet.dat migration fixtures) |
| Wallet + chain restart persistence | partial | `wallet.db` **Pebble** (pure-Go LSM) tx index + scan cursor beside `wallet.json` (`wallet/txdb`); concurrent-read friendly, no cgo/WAL-checkpoint dance on Windows `Close`; live wallet index on each sequential **`ConnectBlock`**; **`wallet_utxo_scan.cache.json`** load/save for balance queries; **`chain_active.manifest.json`** checkpoint with **`utxo.cache`**; catch-up rescan skipped when already indexed through contiguous tip (`node/wallet_catchup_test.go`); **`rescan`** / wallet catch-up skip **`SyncUtxo`** when chainActive covers bodies; startup skips **`SyncUtxo`** when loaded snapshot matches contiguous bodies; caught-up connect worker + solo-miner debounced **`utxo.cache`** save |
| **Core keypool / wallet.dat semantics** | **partial** - **L** | Native BDB read for wallet.dat (`wallet/bdb`, `wallet/corewallet`); **native encrypted decryption** via `CCrypter` scheme (SHA512 master key + AES-256-CBC) with `options.passphrase` for **`ckey`** and **`walletdescriptorckey`**; `dogego_probewalletdat` dry-run + `dogego_importwalletdat` **`keypool_hint`** + **`pool_indices_replayed`** (`encrypted_keys`, `needs_passphrase`, **`pool_count`**, **`pool_pubkeys`**, **`pool_entries`**, **`pool_keys_matched`/`pool_keys_unmatched`** pool-only rows, **`pool_unmatched_entries`**, **`pool_unmatched_hint`**, **`keypool_refill_size`** on import, **`pool_index_min`/`pool_index_max`**); **`keypoolrefill`** fills receive+change **up to** `newsize` (Core TopUpKeyPool; no append-past-target); payment to a still-pooled receive address **consumes** that keypool slot on scan (`ConsumeReceiveKeypoolAddress`); **`wallet/pool_replay.go`** reserves matched BIP44 receive pubkeys into `hd_keypool` on import (skips already-issued index 0 / consumed getnewaddress keys; still stores Core indices) and stores Core indices in **`hd_keypool_core_index`** (`pool_core_indices_stored`; **`getwalletinfo`** / **`dogego_probewalletdat`** / **`getaddressinfo`** / **`validateaddress`** / **`dogego_listwalletaddresses`** when wallet wired; **`iskeypool`** for unused receive **and** change keypool entries; Receive address book keypool tags; **`GET /api/core-wallet-probe`** / **`scripts/core_wallet_workflow.ps1`** **`address_book_keypool_count`** / **`address_book_core_pool_indices_stored`** + **`validateaddress`/`getaddressinfo` `iskeypool` round-trip** + **`pool_core_indices_stored` vs address book count** + **`keypool_topup_ok`** after **`getnewaddress`**); **`dogego cert preflight`/`provision`/`weekly-live`** notes **`wallet_dat_pool_unmatched_hint`** / **`wallet_dat_keypool_refill_size`**; Receive tab + `/api/wallet/probe-walletdat`; **`dogego cert wallet-import`** (offline superset) + **`dogego cert wallet-migration`** (wallet.dat live probe/import); offline synthetic BDB fixtures (unencrypted + encrypted + descriptor-encrypted E2E); dogego-live weekly can require a real fixture via `-require-wallet-dat` |
| **Full wallet RPC + external signer** | **partial** - **L** | UTXO-cache fast path for **`getwalletinfo`**, **`listunspent`**, **`listtransactions`**, **`gettransaction`**, **`listsinceblock`**, **`liststucktransactions`**, **`resendwallettransactions`**; filtered UTXO scans for fund/send; confirmed send **`fee`** + **`hex`** in **`wallet.db`** (block scan, **`sendtoaddress`**, **`sendrawtransaction`** wallet spends, **`bumpfee`**, web Send); non-wallet / pre-index **`hex`** still needs block load when **`tx_index_embed_tx`** false; `enumeratesigners`, `signerdisplayaddress`, `signer_cmd` + PSBT hook in `walletprocesspsbt`; **`getwalletinfo.signer_cmd_configured`** + wallet probe device notes; Milestone E **`psbt_roundtrip_ok`** / **`psbt_bip32_deriv_ok`** probe |
| **Hardware PSBT** | **partial** - **L** | BIP32 deriv paths on fund/process; optional HWI-compatible `signer_cmd` in `walletprocesspsbt` (signer errors propagated; `SignTimeout` on HWI subprocess); mocksigner offline e2e; **`psbt_bip32_deriv_ok`** + **`hardware_psbt_hint`** (incl. when local round-trip succeeds with `signer_cmd` set) on wallet probe / Features card / `core_wallet_workflow.ps1`; native USB/HID without HWI subprocess not in scope |
| Mining (generatetoaddress, aux templates, GBT) | **done** - Digishield GBT bits + BIP22 longpoll; `createauxblock`/`submitauxblock`; `GET /api/core-mining-probe` + `dogego cert mining` + `scripts/core_mining_workflow.ps1`; optional Core GBT compare when tips align |

---

## Post-quantum (design track)

| Area | Status |
|------|--------|
| Recognize FLC1/DIL2/RCG4 + RPC/web Send | partial (MVP done; `dogego cert pq` + `GET /api/core-pq-probe`; web Settings + Send carrier mode) |
| TX_C/TX_R P2SH carrier encode/parse + `dogego_*pqcarrier` RPCs | partial (MVP done; verifier-side PQ only; `pq_carrier` wallet flag; `pqcert/`) |
| Falcon/Dilithium + Raccoon backends (`pqcrypto/`) | partial (Falcon+Dilithium pure-Go; Raccoon-G = vendored Foundation `src/raccoon_g` by [Ed Tubbs](https://github.com/edtubbs) / [Core PR #8](https://github.com/dogecoinfoundation/dogecoin/pull/8) via `CGO_ENABLED=1 -tags raccoon_g`; no placeholder; [CREDITS.md](CREDITS.md)) |
| Mempool relay policy (PQ OP_RETURN + carrier P2SH) | **partial** - `pq_commitment_op_return` / `pq_carrier_p2sh_accept` / `pq_commitment_nonzero_reject` in offline mempool corpus + `TestMempoolAdmissionAcceptsPQ*` |
| Production PQ safety claim | **not claimed** (verifier MVP only) |

---

## Intentionally out of scope (not gaps to close)

- Litecoin parent chain sync (neither Core nor DogeGo; AuxPoW uses embedded parent header in each block)
- **SegWit activation / witness relay** (code present; mainnet disabled like Core; see [SEGWIT_STATUS.md](SEGWIT_STATUS.md); native bech32 is a Core gap, [dogecoin#1760](https://github.com/dogecoin/dogecoin/issues/1760))
- Tor/onion P2P proxy

See [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md).

---

## Try DogeGo (beta)

DogeGo is ready for operators to **test** on reboot testnet or mainnet: sync, wallet, mining, RPC, and the web dashboard. Report what needs tuning.

**Wallet storage note:** the receive/change keypool works like Core’s, but lives in HD **`wallet.json`** (not Core’s `wallet.dat` BDB). You can still **import** a Core `wallet.dat`.

Operator decision tree: [CORE_OPERATOR_RUNBOOK.md](CORE_OPERATOR_RUNBOOK.md).

---

## Why "partial" is not "done" yet (honest scope)

DogeGo is **not a line-by-line port of Dogecoin Core**. It is a **pure-Go reimplementation** with a **production certification bar** that Core never had to meet as a greenfield project:

| Factor | What it means |
|--------|----------------|
| **Exit gate** | Every row in [STANDALONE_FULLNODE_ACCEPTANCE.md](STANDALONE_FULLNODE_ACCEPTANCE.md) must flip from **partial** to **done**, plus multi-hour soak, plus live Core 24/24 on `dogego-live`. |
| **Different storage** | DogeGo does not use Core `blocks/` + `chainstate/` LevelDB; crash/reorg proof is rebuilt via `RebuildFromChain`, Pebble, segment journals - each needs its own soak evidence. |
| **Certification != features** | Much recent work is **operator probes**, offline gates (`dogego cert offline`), and doc drift guards - necessary for sign-off, invisible as "new RPCs". |
| **Live milestones are operator-owned** | Milestones **B/D/E** need `dogego-live` runner, Core side-by-side, 24/24 weekly-live, 60min soak - cannot be closed in code alone. |
| **Intentional non-goals** | Litecoin parent chain for AuxPoW. BIP152 is **partial** (v1 HB relay; auxpow fallback). |
| **Wallet.dat** | Native BDB import works; Core's separate keypool **file** cannot be reproduced 1:1 without storing keys in Core's format (DogeGo uses HD `wallet.json`). |

**What is already strong:** legacy script tests **1059/1059**, **58/58** mempool policy corpus offline, **180+** RPC methods, native wallet.dat migration, web UI, offline cert green.

**What the Features tab measures:** Milestone **E** operator-cert probes (Core compare, dogego-live runner, side-by-side scripts) - **not** "is my solo testnet node broken". Solo testnet without Core beside it should show **Compare** as optional (no `core_rpc_addr`), **Runner** preflight without `-require-core`, **Maintenance** / **End-to-end** as OK with "Syncing (checks OK)" during IBD, and **Mempool** running from embedded testdata (not Skipped in shipped binaries). Full cert still needs Core + `dogego-live`.

**Solo testnet probe fixes (recent):** embedded mempool corpus; runner `RequireCore` only when `core_rpc_addr` set; compare/mempool/maintenance skip Core RPC unless explicitly configured; IBD-tolerant maintenance; offline stateful summary on mempool probe; `GET /api/core-setup-parity`; wallet probe informational notes (empty address book after new address, wallet.dat pool-only rows) no longer yellow warnings; **operator cert solo metrics** (`solo_ok` / `solo_pass` on `/api/core-operator-cert` and summary) count optional/skipped Core gates as pass for solo operators; field evidence probe notes `milestone_a_mainnet_only` on testnet; **solo protocol-lock sanity** on `GET /api/core-compare` (`deployment.protocol_lock` vs consensus params when Core is not configured).

---

## Keeping this doc current

When you close a ROADMAP checkbox or acceptance-matrix row:

1. Update [ROADMAP.md](../ROADMAP.md).
2. Update [STANDALONE_FULLNODE_ACCEPTANCE.md](STANDALONE_FULLNODE_ACCEPTANCE.md) status column.
3. Update `ui/core_parity_gaps.go` (`DefaultCoreParityGaps`) and this file.
4. Run `go test ./ui/...` - `TestDefaultCoreParityGaps` guards non-empty manifest.

---

## Web dashboard mapping (Features tab)

The **Features** tab is the live operator view of this document. Data comes from `GET /api/capabilities`:

| JSON field | UI section |
|------------|------------|
| `parity_summary` | Top cards: standalone exit gate, capability counts (live/partial), gap counts (open/partial/declined), roadmap done/total, RPC class breakdown |
| `core_guidance` | When to use Core vs DogeGo, intentional differences, doc link buttons |
| `certification` | Milestones A/B/D/E, offline test names, operator script paths |
| `core_probe_apis` | Probe API catalog (paths, milestones, bundle membership) |
| `live` | **This node right now** strip (P2P mode, peers, wallet, indexes, DGR, relay policy) |
| `roadmap` | Roadmap highlights checklist |
| `core_parity_gaps` | Core parity backlog (this file in structured form) |
| `categories` | Capability categories with **Open** buttons to relevant dashboard tabs |
| `rpc_methods` | Searchable JSON-RPC table (live / partial / stub from help text) |

Related: [WEB_UI.md](WEB_UI.md) (dashboard tabs), Docs tab section **DogeGo vs Dogecoin Core** (`GET /api/docs`, section `core_parity`).

**Live probes (loopback):**

| Endpoint | Milestone | Purpose |
|----------|-----------|---------|
| `GET /api/core-operator-cert` | E | **17** live web operator-cert gates (incl. Milestone D **setup-parity**, **mining GBT/aux**, **PQ format**, **IBD convergence snapshot**, **addrman snapshot**) + script-only soak rows (`?matrix=1` definitions; `?refresh=1` bypass 90s cache) |
| `GET /api/core-probes` | E | Bundle: compare + maintenance + restart-resume + autostart + founder + runner + **workflow10** + reindex + **BIP152 HB** + **PQ format** + mempool parity + wallet + end-to-end |
| `GET /api/core-autostart-probe` | E | OS login autostart cert gate (`dogego cert autostart`) |
| `GET /api/core-founder-probe` | E | Reboot testnet founder preflight (`dogego cert founder`; skipped OK on mainnet) |
| `GET /api/core-runner-probes` | E | dogego-live CI runner readiness (`dogego cert weekly`; workflow 10) |
| `GET /api/core-workflow10-probe` | E | Workflow 10 preflight (`dogego cert workflow10 -skip-scripts`) |
| `GET /api/core-reindex-probe` | E | Check-only reindex/index workflow (`getrpcinfo`, `getindexinfo`) + optional Core **`getindexinfo`** compare when caught up |
| `GET /api/core-bip152-probe` | E | BIP152 v1: `getpeerinfo` `bip152_hb_to`/`bip152_hb_from` + HB negotiate when caught up + optional Core HB parity notes + **`cmpct_relay_schema_ok`** + `dogego_cmpct_*` relay counters from `getblockchaininfo` |
| `GET /api/core-pq-probe` | E | PQ format/carrier offline probe (FLC1/DIL2/RCG4 OP_RETURN + TX_C/TX_R round-trip; verifier-side only) |
| `GET /api/core-wallet-probe` | E | Wallet workflow: getwalletinfo, getbalance, getnewaddress, validateaddress, address book + keypool/core-pool counts, **`pq_commitments_enabled`/`pq_commitments_ok`**, **`wallet_pq_send_ok`** (PQ OP_RETURN in confirmed send hex), **`pq_carrier_enabled`** when set, setlabel round-trip, enumeratesigners, **`wallet_listtransactions_ok`** (<3s for 40 rows), optional **PSBT round-trip** (`psbt_roundtrip_ok`, **`psbt_bip32_deriv_ok`**) |
| `GET /api/core-maintenance` | E | `verifychain` levels **2** and **4**, `getindexinfo`, `getchaintxstats`, `getblockfilter` (+ Core **`verifychain`** + **`getindexinfo`** + **`getchaintxstats`** + **`getblockfilter`** at shared tip when caught up) |
| `GET /api/core-restart-resume` | E | `rawblocks_sync.json` checkpoint vs contiguous + IBD assist pool |
| `GET /api/core-compare` | E | `getblockchaininfo` + `verifychain` + `gettxoutsetinfo` + **`getmempoolinfo.size`** + **`getmempoolinfo.fullrbf/minrelaytxfee/incrementalrelayfee`** + **`getnetworkinfo.version`** + **`getdeploymentinfo`** + **`chainwork`** + **`mediantime`** + **`verificationprogress`** when caught up + softforks (protocol-lock check) vs Core; DogeGo-only **`dogego_connect_lag`** + **`connect_lag_ok`** when stored bodies lead chainActive |
| `GET /api/mempool/parity-probe` | D | Stateless `testmempoolaccept` rows (32); `offline_corpus` (58 templates) + `stateful_live` (incl. **`setup_parity_*`**) + offline stateful summary; Core side-by-side when Core explicitly configured |
| `GET /api/mempool/stateful-status` | D | Read-only **`offline_corpus`** (58 templates) + stateful offline corpus (26; 24 live-mapped) + live 24/24 gate hints + **`setup_parity_*`** on reboot testnet |
| `GET /api/core-setup-parity` | D | Read-only reboottestnet setup parity check (`dogego cert setup-parity`; no `-mine-bootstrap`) |
| `GET /api/core-end-to-end-probe` | E | Bundled workflow (`core_end_to_end_workflow.ps1` mandatory steps incl. **`offline_corpus`**, **`bip125_offline`**, **`mempool_parity`**, **`ibd_convergence`**, `protocol_lock` + `bip152_hb` + **`pq_format`**) |
| `GET /api/core-status` | E | Cached operator cert snapshot + Core RPC config + **`mempool_offline_corpus`** / **`mempool_parity_*`** when probe cache warm (no probe run) |
| `POST /api/core-test` | E | Quick Core JSON-RPC reachability from Settings form |
| `GET /api/core-cert` | A-E | Milestone manifest + offline `go test` / script names |

**Overview / Settings / sync dock (loopback):**

| UI | Data source |
|----|-------------|
| Overview → Network operator cert | `/api/summary` `dogego_operator_cert_*` (incl. `dogego_operator_cert_solo_ok` / `dogego_operator_cert_solo_pass`) or `/api/core-status` (60s poll during IBD) |
| Overview → Mempool corpus (cached probes) | `/api/summary` `dogego_mempool_offline_corpus_*` + `dogego_mempool_parity_*`; `/api/core-status` live map `mempool_offline_corpus_*` |
| Overview → Sync UTXO body replay | `/api/summary` `dogego_utxo_body_replay_*` when snapshot replay active |
| Settings → Advanced cert strip | `/api/core-status` on tab open |
| Sync dock operator cert `N/M` | Cached cert via summary or `/api/core-status` |
| Startup warm cache | Background `WarmCoreProbeCache` ~8s after web UI listen |
