# DogeGo developer guide

Where everything lives, how to build and test, and how to add features without getting lost.

**Start here if you are new.** Operators should use [OPERATOR.md](OPERATOR.md) and [DOCUMENTATION.md](DOCUMENTATION.md) instead.

---

## Repository layout

```text
DogeGo/
├── cmd/dogego/          CLI entry (node, genesis, ping, …)
├── chain/               Network identity: magic, ports, genesis, seeds, checkpoints
├── config/              dogecoinconf.json schema, merge, validation, RPC defaults
├── consensus/           Header/block validation, script, mempool policy, eras
├── pow/                 Scrypt PoW, compact bits, genesis header helper
├── wire/                Bitcoin/Dogecoin P2P message codec
├── p2p/                 DNS + fixed seed discovery
├── node/                Run loop: sync, IBD, peers, connect catch-up, web hooks
├── store/               Headers journal, raw blocks, UTXO cache, indexes, filters
├── mempool/             In-memory transaction pool
├── rpc/                 JSON-RPC HTTP server + method handlers
├── wallet/              Built-in HD wallet (optional)
├── wallet/bdb/          Berkeley DB reader for Core wallet.dat
├── wallet/corewallet/   Core wallet.dat extract/decrypt (CCrypter)
├── walletmigration/     Offline cert for wallet.dat probe → import
├── fieldevidence/       Milestone A mainnet field-evidence cert suites
├── offlinegate/         Shared offline CI/cert suite list + testdata bootstrap
├── runner/              dogego-live preflight + provision helpers
├── version/             Client version string + GitHub release auto-update
├── desktop/             System tray, dogecoin:// URL handler, BIP21 open
├── ui/                  Web dashboard HTTP server + static assets
├── indexer/             Analytics sidecar (Pebble)
├── scripts/             Operator/CI helpers - prefer `dogego cert` (see scripts/README.md); `.ps1` + some `.sh`
├── docs/                Markdown documentation (this folder)
├── assets/              Icons, static branding
├── LICENSE              MIT (see upstream Bitcoin/Dogecoin Core in file)
└── dogedata/            Default datadir (gitignored; created at runtime)
```

Parent repo **[Dogecoin Core](https://github.com/dogecoin/dogecoin)** (C++) is the **normative spec** for consensus and chain params. DogeGo lives in **[github.com/qlpqlp/dogego](https://github.com/qlpqlp/dogego)**; chain params are mirrored in Go under `chain/` and `consensus/` (see [CHAIN_PARAMETERS.md](CHAIN_PARAMETERS.md)).

---

## Chain & network parameters

**Not one file.** See **[CHAIN_PARAMETERS.md](CHAIN_PARAMETERS.md)** for the full map.

| Need to change… | Go to… |
|-----------------|--------|
| P2P magic / port / genesis | `chain/testnet.go`, `chain/params.go` |
| DNS seeders | `chain/params.go` (mainnet built-in) + `config` merge |
| Fixed seed IPs | `chain/mainnet_seeds.go`, `chain/testnet_seeds.go` (regen via `_gen_seeds.py`) |
| Checkpoints | `chain/checkpoints.go` |
| Digishield / AuxPoW heights | `consensus/dogeconsensus.go` |
| Assume-valid default | `consensus/assumevalid.go` |
| Operator network selection | `dogecoinconf.json` `"network"` |

---

## Build & run

```powershell
cd DogeGo
go build -o dogego.exe ./cmd/dogego
.\dogego.exe node
```

First run without datadir opens the **setup wizard** (Web UI). Config file: `%APPDATA%\DogeGo\dogecoinconf.json` (legacy `%APPDATA%\dogego\` still read) or `dogecoinconf.json` in repo / datadir.

**Plain HTTP (DogeBox):** `.\dogego.exe node -notls` or `DOGEGO_NO_TLS=1` — skips local HTTPS / CA install. See [WEB_UI.md](WEB_UI.md#local-https-and--notls).

**Raccoon-G in releases:** GitHub Actions does **not** cross-compile CGO. Each OS builds on a native runner with GMP/MPFR (`CGO_ENABLED=1 -tags raccoon_g`). Why: [pqcrypto/raccoon_g/BUILD.md](../pqcrypto/raccoon_g/BUILD.md). The Foundation in-tree port is by [Ed Tubbs](https://github.com/edtubbs) ([@EdTubbs](https://x.com/EdTubbs)); see [CREDITS.md](CREDITS.md). Local Windows `build.ps1` stays pure-Go; use a Release binary or `./build_raccoon.sh` for Raccoon.

Debug from VS Code: use **Go: launch** on `cmd/dogego` (see repo `.vscode/launch.json` if configured).

---

## Package guide

| Package | You touch this when… |
|---------|----------------------|
| **`cmd/dogego`** | Adding CLI subcommands, flags, process lifecycle, restart/update spawn |
| **`version`** | Client version display, GitHub release polling, SHA256 verify, update download |
| **`desktop`** | System tray, URL protocol handler, open dashboard helpers |
| **`chain`** | New network, genesis, seeds, checkpoints, address versions |
| **`config`** | New `dogecoinconf.json` fields, defaults, validation |
| **`consensus`** | Validation rules, script, assumevalid, mempool admission |
| **`pow`** | Header hashing, difficulty encoding |
| **`wire`** | New P2P message types, block/tx parsing, **BIP152** compact blocks (`cmpctblock.go`, `cmpct_reconstruct.go`) |
| **`p2p`** | Peer discovery algorithm (not multi-peer policy - that's `node/`) |
| **`node`** | Sync state machine, IBD, peer manager, connect catch-up, recovery, **BIP152 HB relay** (`cmpct.go`), **DGR CGNAT relay** (`node/dgr/`) |
| **`store`** | On-disk formats, UTXO cache, header segments, corruption recovery |
| **`mempool`** | Relay policy, RBF, package limits |
| **`rpc`** | JSON-RPC methods, `getblockchaininfo` diagnostics |
| **`wallet`** | Keys, PSBT, send/receive |
| **`wallet/bdb`**, **`wallet/corewallet`** | Native Core `wallet.dat` probe/extract (plain + encrypted) |
| **`walletmigration`** | Wallet.dat migration offline cert suites (`dogego cert wallet-migration`) |
| **`operatorworkflow`** | Milestone E standalone operator cert (`dogego cert operator`) |
| **`walletimport`** | BIP39/BIP38 + signer + wallet.dat import cert (`dogego cert wallet-import`) |
| **`pqcert`** | PQ OP_RETURN + TX_C/TX_R carrier format cert (`dogego cert pq`; no production PQ safety claim) |
| **`fieldevidence`** | Milestone A field-evidence offline cert suites (`dogego cert field-evidence`) |
| **`offlinegate`** | Offline cert suite list + testdata bootstrap (`dogego cert offline`, CI scripts) |
| **`ibdconvergence`** | Forward IBD progress cert (`dogego cert ibd-convergence`; mirrors `scripts/ibd_convergence_check.ps1`) |
| **`ui`** | Dashboard APIs, setup wizard, docs manifest (`docs_index.go`) |
| **`scripts`** | Operator automation (health checks, soak gates, Core compare) |

Data flow: **`cmd/dogego` → `node/run.go` →** `store` + `consensus` + `wire`/`p2p` + `rpc` + `ui`.

---

## Adding a JSON-RPC method

1. Implement handler in `rpc/<method>.go` (or existing file).
2. Register in **`rpc/dispatch.go`** (`SupportedMethods`).
3. One-line help in **`rpc/help.go`**.
4. Test in **`rpc/*_test.go`**.
5. Document in **`docs/RPC.md`** with example `curl`.
6. Optional: **`rpc/cookbook.go`**, **`rpc/openrpc.go`** for integrators.
7. Checkbox in **`ROADMAP.md`** when MVP-complete.
8. Update **`ui/docs_index.go`** if operator-facing narrative changes.

---

## Adding consensus or sync behavior

1. Read matching logic in **`../src/`** (Core).
2. Implement in **`consensus/`** or **`node/`**.
3. Add tests with vectors from `consensus/testdata/` or Core exports.
4. Document intentional differences in **`docs/INTENTIONAL_DIFFERENCES.md`**.
5. Update **`docs/CORE_PARITY_GAPS.md`** / **`ui/core_parity_gaps.go`** status.

IBD policy hotspots: `node/ibd.go`, `node/connect_catchup.go`, `node/rawsync_progress.go`, `node/block_assist.go`.

---

## Configuration layers

| Layer | Location | Purpose |
|-------|----------|---------|
| Chain identity (hardcoded) | `chain/`, `consensus/` | Magic, genesis, eras - see [CHAIN_PARAMETERS.md](CHAIN_PARAMETERS.md) |
| Operator config | `dogecoinconf.json` | Network, RPC, peers, indexes, wallet, assumevalid |
| CLI flags | `cmd/dogego/main.go` | Override config for one run |
| Environment | `DOGEGO_*`, `DOGECOINCONF` | Scripts, RPC port, datadir |
| Browser-only | `localStorage` | Web UI display prefs (not on disk) |

---

## Tests

```powershell
go test ./...                    # full suite (can take several minutes)
go test ./node/... -run IBD      # focused
go test ./consensus/...          # script/mempool vectors
go test ./rpc/...                # RPC handlers
```

### Offline certification (no live node)

Cross-platform bundle shared by **`dogego cert offline`**, **`scripts/ci_offline_gate.ps1`**, **`scripts/ci_offline_gate.sh`**, and the GitHub Actions **`offline`** job (`.github/workflows/dogego.yml` at repo root). Suite list and consensus/testdata bootstrap live in **`offlinegate/`** (`suites.go`, `bootstrap.go`) so CLI and CI scripts stay aligned.

**Protocol lock:** mainnet follows Dogecoin Core with **no consensus fork**; offline gates verify differential tests against Core semantics and do **not** certify a new protocol. Mainnet rule changes require Core-aligned review and new harness vectors (see ROADMAP **Dogecoin protocol lock**).

Milestone A field evidence: **`fieldevidence/`** (`suites.go`, `run.go`). Milestone E standalone operator cert: **`operatorworkflow/verify.go`**. Wallet migration: **`walletmigration/verify.go`**. Wallet import superset: **`walletimport/verify.go`**.

```powershell
cd DogeGo
go run ./cmd/dogego cert offline
go run ./cmd/dogego cert field-evidence   # Milestone A mainnet field block/header gates
go run ./cmd/dogego cert wallet-migration   # BDB probe/extract/import fixtures
go run ./cmd/dogego cert wallet-migration -offline-only   # offline suites only (wallet_migration_cert.ps1)
go run ./cmd/dogego cert wallet-import      # BIP39/BIP38 + signer + wallet.dat (superset; mirrors wallet_import_cert.ps1)
go run ./cmd/dogego cert operator           # Milestone E deep cert (core + field-evidence + wallet-import; ~5-20 min)
go run ./cmd/dogego cert pq                 # PQ format/carrier cert; no production PQ safety claim
go run ./cmd/dogego cert wallet-migration -skip-offline -live-import -require-wallet-dat   # dogego-live RPC import
.\scripts\ci_offline_gate.ps1               # same gates as CI
./scripts/ci_offline_gate.sh                # Linux/macOS
.\scripts\field_evidence_cert.ps1            # PowerShell equivalent of cert field-evidence
./scripts/field_evidence_cert.sh
.\scripts\operator_workflow_cert.ps1         # PowerShell equivalent of cert operator (+ optional live disk connect)
./scripts/operator_workflow_cert.sh
.\scripts\wallet_import_cert.ps1             # PowerShell equivalent of cert wallet-import
./scripts/wallet_import_cert.sh
.\scripts\wallet_migration_cert.ps1          # offline + live wallet.dat (see -SkipOffline)
./scripts/wallet_migration_cert.sh
.\scripts\pq_cert.ps1                        # PowerShell equivalent of cert pq
./scripts/pq_cert.sh                         # Linux/macOS equivalent of cert pq
./scripts/cert_offline_prerequisites.sh      # ROADMAP offline prerequisite bundle
```

Wallet migration cert covers synthetic Berkeley DB fixtures (plain + encrypted `wallet.dat`), RPC `dogego_importwalletdat` / `dogego_probewalletdat` (pool metadata, **`pool_keys_matched`/`pool_keys_unmatched`**, **`pool_unmatched_hint`**, **`keypool_refill_size`**, **`pool_indices_replayed`** via **`wallet/pool_replay.go`** on HD import, **`keypool_hint`**), and end-to-end import into the DogeGo wallet. Pool-only rows (`pool_keys_unmatched`) cannot be recovered without Core spend keys - see [OPERATOR.md](OPERATOR.md) § Core wallet.dat keypool. **`GET /api/core-wallet-probe`** includes optional `dogego_probewalletdat` when **`DOGEGO_WALLET_DAT`** is set (pool metadata + `pool_keys_unmatched` warning + `pool_unmatched_hint`; **`address_book_keypool_count`** / **`address_book_core_pool_indices_stored`** from `dogego_listwalletaddresses`; mirrors `scripts/core_wallet_workflow.ps1`).

### Live certification (self-hosted runner)

**Before `dogego-live`:** run **Offline prerequisites** (`dogego cert offline` + `dogego cert wallet-import`; use `dogego cert wallet-migration` for wallet.dat live probe/import only) - see [ROADMAP.md](../ROADMAP.md) certification exit checklist.

Requires a running node and/or Dogecoin Core on **`dogego-live`**. Weekly CI uses **RPC probe in preflight** and **RPC import after reboottestnet setup**. Preflight/provision/weekly notes include **`pool_unmatched_hint`** and **`wallet_dat_keypool_refill_size`** when Core pool-only rows remain (`wallet_dat_probe` / `wallet_dat_import` notes; Features → **CI runner readiness**).

```powershell
go run ./cmd/dogego cert preflight -require-core -require-wallet-dat
go run ./cmd/dogego cert provision -preflight -json   # dogego-live checklist (ports, wallet.dat pool probe, setup parity env)
go run ./cmd/dogego cert provision -preflight -run-setup -mine-bootstrap   # checklist + live setup-parity (mirrors -RunSetup)
go run ./cmd/dogego cert setup-parity -mine-bootstrap   # reboottestnet wallet bootstrap (mirrors setup_reboottestnet_core_parity.ps1)
go run ./cmd/dogego cert weekly -require-wallet-dat
go run ./cmd/dogego cert weekly -mine-bootstrap -require-wallet-dat
go run ./cmd/dogego cert weekly-live -mine-bootstrap -require-wallet-dat
go run ./cmd/dogego cert weekly-live -skip-scripts -mine-bootstrap   # preflight-only (no PS1 Core gate)
go run ./cmd/dogego cert workflow10 -skip-provision -mine-bootstrap -require-wallet-dat -json   # workflow 10 preflight (no PS1 gates)
go run ./cmd/dogego cert workflow10 -mine-bootstrap -require-wallet-dat   # full dogego-live sequence (provision → weekly-live)
go run ./cmd/dogego cert workflow10 -enable-github -github-apply -mine-bootstrap -require-wallet-dat -include-live-soak
go run ./cmd/dogego cert live-soak -duration-min 60 -require-soak-env
go run ./cmd/dogego cert live-soak -skip-scripts                     # preflight-only (no multi-hour soak script)
go run ./cmd/dogego cert wallet-migration -skip-offline -live-probe      # probe only
go run ./cmd/dogego cert wallet-migration -skip-offline -live-import     # probe + import
go run ./cmd/dogego cert enable-weekly -require-wallet-dat   # sets DOGEGO_WALLET_DAT_REQUIRED repo var
.\scripts\provision_wallet_dat_fixture.ps1   # locate Core wallet.dat for live probe
.\scripts\wallet_migration_cert.ps1 -SkipOffline   # live RPC import only (dogego-live)
```

GitHub Actions manual dispatch: `gh workflow run dogego.yml -f live_weekly=true -f require_wallet_dat=true`

Full dogego-live sequence: [CORE_SIDE_BY_SIDE_WORKFLOWS.md](CORE_SIDE_BY_SIDE_WORKFLOWS.md) (workflow 10).

Web UI: Features → **Certification** shows **17** live web gates (`GET /api/core-operator-cert`); includes Milestone D **setup-parity**, **BIP152 HB** (`GET /api/core-bip152-probe`), **mining GBT/aux** (`GET /api/core-mining-probe`), **PQ format** (`GET /api/core-pq-probe`), **IBD convergence snapshot** (`GET /api/core-ibd-convergence-probe`), **addrman snapshot** (`GET /api/core-addrman-probe`), and **CI runner readiness** (`GET /api/core-runner-probes`). **Workflow 10 preflight** (`GET /api/core-workflow10-probe`) is included in **Run all probes** (`GET /api/core-probes`) and the probe strip mini-pill. **Run all probes** on Features also fills the probe strip mini-pills.

### Solo testnet Features probes (no Core required)

The **Live Core probes** strip on Features is Milestone **E/D cert oriented**, not a health check for a solo node:

| Probe | Solo testnet without Core |
|-------|---------------------------|
| Compare | Optional (no `core_rpc_addr`) |
| Maintenance / Reindex | OK with **Syncing (checks OK)** during IBD |
| Runner | OK on reboot testnet (no `-require-core`) |
| Mempool | Stateless rows + **offline stateful** + **stateful_live** summary |
| Wallet | OK; informational **notes** (address book lag, wallet.dat pool-only rows) |
| End-to-end | OK when maintenance/reindex are OK during sync |
| Operator cert (Overview / sync dock) | Shows **solo** N/17 when optional Core gates (compare, runner without Core, setup-parity skipped on mainnet) count as pass; strict **live** count still needs Core for Milestone E |

Useful loopback APIs while developing:

```text
GET /api/core-probes?refresh=1          # full bundle
GET /api/mempool/stateful-status        # Milestone D offline + live gate hints
GET /api/core-setup-parity              # reboottestnet bootstrap check (read-only)
```

See [CORE_PARITY_GAPS.md](CORE_PARITY_GAPS.md) § *Why partial is not done yet*.

### Solo testnet wallet RPC (many coinbases)

| API | Fast path |
|-----|-----------|
| `GET /api/wallet`, `/api/wallet/txs`, `/api/wallet/utxos` | UTXO cache + SpendScripts (`FilterRowsByScriptSet`); **`wallet_scan_index_ok`** / **`wallet_history_fast_path`** / **`wallet_listtransactions_utxo_walk`** / **`wallet_listtransactions_scan_pending`** on GET `/api/wallet` and **`GET /api/summary`** when scan metadata wired; History tab defers heavy fetch while scan builds index (>64 UTXOs); **`type=sent`** and **`type=all`** (with scan index) use **`wallet.db`** for sends + fee/hex |
| `GET /api/blockstep/address` | Addr index when present; else **`ScanAddressWalletFast`** (UTXO + **`wallet.db`**) for wallet-owned addresses before raw block window walk |
| `POST /api/wallet/rescan` | Async block rescan via RPC `rescan` (incremental or `{full:true}`); dashboard auto-starts incremental rescan once per browser session when caught up and (**`needs_rescan`** or **`wallet_listtransactions_utxo_walk`** with >64 UTXOs) |
| `getwalletinfo`, `getbalance`, `listunspent` | UTXO cache; `wallet_utxo_scan.cache.json` on disk (refreshed on connect advance); **`wallet_index_height`**, **`needs_rescan`**, **`dogego_wallet_scan_index_ok`**, **`dogego_wallet_history_fast_path`**, **`dogego_wallet_listtransactions_utxo_walk`**, **`dogego_wallet_listtransactions_scan_pending`**, **`dogego_wallet_history_deferred`**, **`dogego_wallet_history_defer_reason`**, **`scanning`** when scan metadata wired |
| `walletcreatefundedpsbt`, `walletprocesspsbt` | Milestone E PSBT round-trip on **`GET /api/core-wallet-probe`** / `core_wallet_workflow.ps1` when unlocked with mature balance; **`psbt_bip32_deriv_ok`** via `decodepsbt`; **`keypool_topup_ok`** after `getnewaddress` on HD wallets |
| `listtransactions`, `gettransaction`, `listsinceblock` | UTXO-cache light rows + 20s cache (`walletUIRowsCached`); skips UTXO receive walk when **`wallet.db`** has receive rows; receive mining rows use vout-0 heuristic when `tx_index_embed_tx` is false (`walletReceiveTxKind`); send **`fee`** + **`hex`** from **`wallet.db`** when indexed (`WalletSendFeeLookup`, `WalletTxHexLookup`; `TestExecListTransactionsSendFeeFromWalletDB`, `TestExecGetTransactionWalletHexAndFeeFromWalletDB`, **`TestExecListSinceBlockSendFeeFromWalletDB`**, **`TestExecListTransactionsWalletManyUtxosUsesScanIndex`**) |
| **`liststucktransactions`**, **`resendwallettransactions`** | Same light row cache |

Deep tx-index hex on old sends may still load blocks when `"tx_index_embed_tx": false` and the tx was never wallet-indexed; wallet **`gettransaction` hex** uses **`wallet.db`** cache (`WalletTxHexLookup`) after mempool when present (broadcast + block scan). Web **`/api/wallet/txs`** sent rows lazy-load hex from tx index or block height when missing from **`wallet.db`** (and persist into **`wallet.db`**). **`rescan`** / startup wallet catch-up skip **`SyncUtxo`** when chainActive already covers contiguous bodies. Confirmed send **`fee`** uses the same index (`fee_koinu`, `WalletSendFeeLookup`).

Other certification scripts: `scripts/operator_workflow_cert.ps1`, `scripts/core_operator_workflow_cert.ps1`, `scripts/ibd_live_soak_gate.ps1`, `scripts/wallet_migration_cert.ps1`.

---

## Documentation maintenance

When you change behavior, update **at least one** of:

| Audience | Update |
|----------|--------|
| Operators | `docs/OPERATOR.md`, `docs/CORE_OPERATOR_RUNBOOK.md` |
| Integrators | `docs/RPC.md`, `docs/INTEGRATION.md` |
| Contributors | `docs/DEVELOPER_GUIDE.md`, `docs/CHAIN_PARAMETERS.md`, `docs/ARCHITECTURE.md` |
| Web UI Docs tab | `ui/docs_index.go` |
| Parity tracking | `docs/CORE_PARITY_GAPS.md`, `ui/core_parity_gaps.go` |

Index: **`docs/DOCUMENTATION.md`**.

---

## License headers

New source files should include the MIT header. Run:

```powershell
.\scripts\apply_mit_license.ps1
```

See **`LICENSE`** and **`README.md`** § License.

---

## Key reference files

| Topic | File |
|-------|------|
| Node startup | `node/run.go` |
| Release auto-update | `version/updatecheck.go`, `version/update_download.go`, `ui/update_api.go` |
| Detached restart / apply | `cmd/dogego/restart_spawn*.go`, `cmd/dogego/update_replace.go` |
| `getblockchaininfo` | `rpc/chaininfo_helpers.go` |
| IBD body download | `node/ibd.go`, `node/fetch.go` |
| Connect catch-up | `node/connect_catchup.go`, `node/connect_catchup_worker.go` |
| Peer manager | `node/peermgr.go`, `node/addrbook.go` |
| Config merge | `config/merge.go` |
| Core chain spec | [github.com/dogecoin/dogecoin](https://github.com/dogecoin/dogecoin) `src/chainparams.cpp` |
| Seed codegen | `chain/_gen_seeds.py` |

---

## Getting help

- **Architecture overview:** [ARCHITECTURE.md](ARCHITECTURE.md)
- **Core vs DogeGo:** [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md)
- **What’s left for production parity:** [STANDALONE_FULLNODE_ACCEPTANCE.md](STANDALONE_FULLNODE_ACCEPTANCE.md)
- **Roadmap phases:** [ROADMAP.md](../ROADMAP.md)
