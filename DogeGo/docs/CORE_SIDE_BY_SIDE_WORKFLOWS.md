# Core vs DogeGo side-by-side operator workflows

Use this when validating DogeGo as a **standalone full node** against Dogecoin Core on the same machine. Replace paths and RPC ports with your setup.

## Prerequisites

| Item | Core | DogeGo |
|------|------|--------|
| Binary | `dogecoin-qt` / `dogecoind` | `dogego.exe` |
| Datadir | `%APPDATA%\Dogecoin` | `dogecoinconf.json` → `datadir` |
| RPC port | default `22555` | `rpc` in config - use **`:22557`** when Core already uses `:22555` |
| Network | `-testnet` or mainnet | `network=testnet` or `mainnet` |

## Workflow 1: Offline certification (no P2P)

DogecGo automated checks (no Core equivalent in-tree):

```powershell
cd C:\path\to\dogecoin\DogeGo
.\scripts\operator_workflow_cert.ps1
```

Optional long-haul wrapper (offline gates + optional live RPC when `DOGEGO_IBD_SOAK=1`):

```powershell
.\scripts\ibd_soak_cert.ps1
```

Full Milestone E bundle (offline + live + optional Core compare + IBD convergence):

```powershell
$env:DOGEGO_IBD_SOAK = "1"
$env:DOGEGO_IBD_CONVERGE = "1"   # 2-minute body progress window
$env:DOGEGO_CORE_COMPARE = "1"   # requires dogecoin-cli
.\scripts\core_operator_workflow_cert.ps1
```

Prove forward body IBD over a time window (node must be running):

```powershell
$env:DOGEGO_RPC_PORT = "22557"   # when Core uses :22555
.\scripts\node_health.ps1
.\scripts\ibd_convergence_check.ps1
.\scripts\ibd_convergence_check.ps1 -IntervalSec 300 -MinRawProbeAdvance 100
```

## Workflow 2: Chain health after local sync

When **Dogecoin Core already owns RPC `:22555`**, start DogeGo on a separate port and compare in one step:

```powershell
cd C:\path\to\dogecoin\DogeGo
.\scripts\core_compare_with_core.ps1
# DogeGo on :22557, Core on :22555; optional mempool corpus:
.\scripts\core_compare_with_core.ps1 -MempoolProbe
# DogeGo already running on :22557:
.\scripts\core_compare_with_core.ps1 -SkipStart
```

Automated side-by-side probe (both nodes must be running on the **same network**):

```powershell
cd C:\path\to\dogecoin\DogeGo
$env:DOGEGO_RPC_PORT = "22557"
$env:DOGEGO_CORE_RPC_PORT = "22555"
.\scripts\core_parity_probe.ps1
```

Web UI: Features → **Core compare** (`GET /api/core-compare`) or **Run all probes**. When `core_rpc_addr` is set and Core is reachable, the compare card shows chain tips, `verifychain`, UTXO set height/hash, **`getmempoolinfo.size`** (informational), **`getnetworkinfo.version`**, and DogeGo-only **`dogego_connect_lag`** when stored bodies are ahead of chainActive.

Or bundled with IBD soak when Core is available:

```powershell
$env:DOGEGO_IBD_SOAK = "1"
$env:DOGEGO_CORE_COMPARE = "1"
.\scripts\ibd_soak_cert.ps1
```

Set `DOGEGO_CORE_RPC_PORT` when Core and DogeGo use different RPC ports on one machine.

Stateless **`testmempoolaccept`** side-by-side (**32** rows in `consensus/testdata/mempool_parity_rpc.json`; full 58-vector gate offline):

```powershell
.\scripts\core_mempool_parity_probe.ps1
```

Or with full soak: `$env:DOGEGO_MEMPOOL_PROBE = "1"` alongside `DOGEGO_IBD_SOAK=1` and `DOGEGO_CORE_COMPARE=1`.

| Step | Core | DogeGo |
|------|------|--------|
| Block count | `dogecoin-cli getblockcount` | `dogego-cli getblockcount` or web Console |
| Verification progress | `getblockchaininfo` → `verificationprogress` | same field on `getblockchaininfo` |
| Verify stored chain | `verifychain 4 0` | `verifychain 4 0` (level 4 uses `rawblocks/` + tx index) |
| UTXO set hash | `gettxoutsetinfo` → `hash_serialized` | same (`gettxoutsetinfo`) |

**Pass criteria:** `verifychain` returns `true`; `gettxoutsetinfo.height` matches `getblockcount`.

## Workflow 3: Mempool policy (dry-run)

Automated stateless probe (both nodes running):

```powershell
.\scripts\core_mempool_parity_probe.ps1
```

| Step | Core | DogeGo |
|------|------|--------|
| Test tx admission | `testmempoolaccept '["<hex>"]'` | same RPC |
| Reject reason shape | `reject-reason` string | same field (see `consensus/MempoolRejectReason`) |

**Pass criteria:** Same `allowed` / `reject-reason` class for **32** stateless live rows (`mempool_parity_rpc.json`); full **58**-vector gate offline (`go test ./consensus -run TestCoreMempool`).

## Workflow 4: Header journal recovery

| Situation | Core | DogeGo |
|-----------|------|--------|
| Stale headers / bad peer | Manual delete `blocks/index` or reindex | Automatic rewind + `dogego_recoverheaders` |
| Operator rewind | `invalidateblock` + reconsider | `truncatetoheight <N>` or web Recover |

**Pass criteria:** Node resumes forward sync without manual file surgery when corruption is within tested repair classes (see `node/auto_recovery_test.go`, `store/journal_tail_repair_test.go`).

## Workflow 4b: Post-aux header stall (~510k / ~8% UI)

When header progress stops near **510000** on mainnet while block bodies may still advance, upgrade DogeGo (Core-parity aux parent chain ID - reject only parent encoding Dogecoin `0x62`):

```powershell
cd C:\path\to\dogecoin\DogeGo
go build -o dogego.exe .\cmd\dogego
# stop old dogego, start new binary, then:
.\scripts\upgrade_post_aux_verify.ps1
.\scripts\watch_sync.ps1
# optional strict gate:
.\scripts\upgrade_post_aux_verify.ps1 -RequireHeadersPast510k
```

| Check | DogeGo RPC / UI |
|-------|-----------------|
| Build has fix | `getblockchaininfo` → `dogego_auxpow_parent_chain_id_core_parity: true` |
| Stall band | `dogego_post_aux_era_header_stall: true` while tip 509.5k-510.5k and catch-up pending |
| Operator hint | `dogego_header_sync_recovery` + Overview banner |
| Progress | `headers` climbs past **510000** within minutes (no `headers/` wipe required) |

**Pass criteria:** `dogego_auxpow_parent_chain_id_core_parity` is true; `headers` > 510000 after restart; logs no longer repeat `aux parent chain id must be zero`.

See [CORE_OPERATOR_RUNBOOK.md](CORE_OPERATOR_RUNBOOK.md) § “Header sync stuck near ~8%”.

## Workflow 5: Truncate and resync bodies

Destructive maintenance (testnet/dev only unless you understand the blast radius):

```
truncatetoheight 1000
```

DogecGo truncates `headers.bin`, prunes `rawblocks/` above height, rebuilds tx index slice, and replays UTXO through `RebuildUtxoThrough`.

**Pass criteria:** `getblockcount` == truncate height; `verifychain 4 0` true; `gettxoutsetinfo` consistent after replay.

## Automated RPC integration tests (DogecGo only)

```powershell
go test ./rpc -run "TestExecVerifyChainLevel4|TestExecGetTxOutSetInfo|TestExecTestMempoolAcceptDifferential" -count=1
```

These use reboot-testnet fixtures with stored blocks - not a substitute for mainnet IBD soak.

## Web dashboard (loopback)

When the DogeGo dashboard runs on `127.0.0.1`, use these instead of PowerShell for live Milestone E gates:

| Operator goal | Web UI | API |
|---------------|--------|-----|
| All live probes | Features → **Run all probes** | `GET /api/core-operator-cert?refresh=1` |
| Cached cert + mempool corpus (lightweight) | Overview → Network; Settings → Advanced | `GET /api/core-status` |
| End-to-end bundle (`offline_corpus`, `bip125_offline`, `mempool_parity`, `mining`) | Features → End-to-end card | `GET /api/core-end-to-end-probe` |
| Mempool parity (32 rows) | Features / Mempool → Policy | `GET /api/mempool/parity-probe` |
| Wallet basics (Milestone E) | Features → Wallet probe card | `GET /api/core-wallet-probe` |
| Mining GBT / aux (Milestone E) | Features → Mining probe card | `GET /api/core-mining-probe` |
| Core RPC reachability | Settings → Advanced → Test Core | `POST /api/core-test` |

Startup **warms the probe cache** (~8s after RPC dispatch) so `/api/summary` shows `dogego_operator_cert_*` and `dogego_mempool_*` without manual clicks. Overview polls `/api/core-status` every 60s during IBD.

See [CORE_OPERATOR_RUNBOOK.md](CORE_OPERATOR_RUNBOOK.md) and [WEB_UI.md](WEB_UI.md).

## Wallet basics probe (Milestone E)

When the built-in wallet is enabled, verify Core-shaped wallet RPCs without importing:

```powershell
.\scripts\core_wallet_workflow.ps1
# JSON (includes address_book_keypool_count / address_book_core_pool_indices_stored):
.\scripts\core_wallet_workflow.ps1 -Json
```

Web UI: Features tab → **Wallet probe** (`GET /api/core-wallet-probe`). The probe runs `getwalletinfo`, `getbalance`, `getnewaddress`, `validateaddress`, `dogego_listwalletaddresses` (reports **`address_book_count`**, **`address_book_keypool_count`**, **`address_book_core_pool_indices_stored`** when HD keypool/Core pool indices are stored), **`validateaddress`/`getaddressinfo` `iskeypool` round-trip** on the first keypool row, **`pool_core_indices_stored` vs address book count** when `getwalletinfo` exposes stored indices, **`wallet_scan_index_ok`** / **`wallet_history_fast_path`** (partial index with receive rows - listtransactions skips UTXO receive walk), **`wallet_history_defer_reason`** when History would defer (IBD, connect lag, or scan build with >64 UTXOs - skips **`listtransactions`** latency gate), `setlabel` / `getaddressesbylabel` / `listlabels` round-trip, `enumeratesigners`, and optional **PSBT round-trip** (`walletcreatefundedpsbt` + `walletprocesspsbt` for 0.001 DOGE when wallet is unlocked with mature balance; skipped with a note when locked or insufficient funds). When **`DOGEGO_WALLET_DAT`** is set, also runs `dogego_probewalletdat` (pool metadata, **`pool_unmatched_hint`**, **`pool_indices_replayed`** is always false on probe-only).

Solo miners: wallet send, history, and BlockStep address balance use **filtered UTXO-cache scans** (`FilterRowsByScriptSet`) instead of dumping the full chain UTXO set - required for acceptable latency with hundreds of coinbase UTXOs. When **`wallet.db`** has receive rows, **`listtransactions`** and **`GET /api/wallet/txs?type=all`** prefer the scan index even if **`needs_rescan`** is still set (`wallet_history_fast_path`). Fresh wallets without scan rows report **`dogego_wallet_listtransactions_utxo_walk`** until rescan builds receive history; **`wallet_listtransactions_scan_pending`** while rescan runs before first receive rows. **`GET /api/wallet/txs`** and the wallet probe skip heavy history during the same defer windows as the dashboard History tab.

## Mining GBT / aux probe (Milestone E)

Verify Digishield `getblocktemplate` fields (incl. BIP22 `longpollid`), `getmininginfo`, and `createauxblock` once the tip is in the AuxPoW era (mainnet ≥ 371337, reboot testnet ≥ 158100):

```powershell
.\scripts\core_mining_workflow.ps1 -DogeGoOnly
# Optional Core GBT side-by-side when tips align:
.\scripts\core_mining_workflow.ps1
# Web probe (dashboard must be up):
.\scripts\core_mining_workflow.ps1 -WebProbe
```

Offline: `dogego cert mining`. Web UI: Features → **Mining / GBT / aux probe** (`GET /api/core-mining-probe`). Live operator-cert gate id: **`mining`** (17th web gate).

## Not yet in side-by-side scope

- Full `script_tests.json` interpreter parity
- `submitpackage` / package RBF edge cases vs Core 26.x (offline CPFP package feerate + `replaced-transactions` / `effective-includes` done; live Core compare still open)
- Core keypool file semantics (pre-generated keypool in separate `wallet.dat` pool)
- Native USB/HID hardware signers without HWI subprocess

## Workflow 9: Core wallet.dat migration (DogeGo)

Migrate keys from a Core Berkeley DB `wallet.dat` without switching back to Core for unencrypted or passphrase-encrypted legacy wallets.

**Probe (no import):**

```powershell
dogego-cli dogego_probewalletdat "C:\Users\…\AppData\Roaming\Dogecoin\wallet.dat"
```

Returns `encrypted`, `encrypted_keys`, `pool_count`, `pool_pubkeys`, `pool_entries` (with `spends_key_matched` when a pool pubkey has a spend key), `pool_keys_matched`, `pool_keys_unmatched`, `pool_unmatched_entries`, `pool_unmatched_hint`, `pool_index_min`/`pool_index_max`, `pool_indices_replayed`, `needs_passphrase`, `can_import`, and `hint`.

**Import (native, unencrypted):**

```powershell
dogego-cli dogego_importwalletdat "C:\path\to\wallet.dat"
```

Native import responses may include `pool_count`, `pool_pubkeys`, `pool_entries`, `pool_keys_matched`, `pool_keys_unmatched`, `pool_unmatched_entries`, `pool_unmatched_hint`, `keypool_hint`, `keypool_refill_size` when pool-only rows remain, `pool_indices_replayed` (true after import when matched HD receive pubkeys replay into `hd_keypool` via `wallet/pool_replay.go`; false on probe-only), `pool_core_indices_stored`, and `pool_index_min`/`pool_index_max` when Core keypool entries are present; spend keys import via `ckey`/`key` and HD wallets run `keypoolrefill`.

**Import (native, encrypted - Core CCrypter):**

```powershell
dogego-cli dogego_importwalletdat "C:\path\to\wallet.dat" '{"passphrase":"yourpass"}'
```

**Import (Core fallback when Core RPC is configured):**

```powershell
dogego-cli dogego_importwalletdat "C:\path\to\wallet.dat" '{"via_core_rpc":true}'
```

**Web UI:** Receive tab → **Core wallet.dat** card (path, optional passphrase, Probe + Import).

**Offline certification:**

```powershell
dogego cert wallet-migration
# or Windows scripts:
.\scripts\wallet_migration_cert.ps1
# Optional live file probe (no running node required):
dogego cert wallet-migration -wallet-dat "C:\path\to\wallet.dat" -passphrase "yourpass"
# or:
$env:DOGEGO_WALLET_DAT = "C:\path\to\wallet.dat"
$env:DOGEGO_WALLET_DAT_PASSPHRASE = "yourpass"
.\scripts\wallet_migration_cert.ps1
```

See [WALLET.md](WALLET.md) and `scripts/core_wallet_workflow.ps1` (optional `DOGEGO_WALLET_DAT` probe when wallet RPC is enabled).

**dogego-live runner fixture** (optional weekly gate):

```powershell
.\scripts\provision_wallet_dat_fixture.ps1 -CoreDataDir "C:\path\to\core\datadir" -SetUserEnv
$env:DOGEGO_WALLET_DAT_REQUIRED = "1"
dogego cert weekly -require-wallet-dat
```

## Workflow 6: ZMQ (DogeGo)

Configure in `dogecoinconf.json` (restart required):

```json
"zmqpubhashblock": "tcp://127.0.0.1:28332",
"zmqpubhashtx": "tcp://127.0.0.1:28333",
"zmqpubrawblock": "tcp://127.0.0.1:28334",
"zmqpubrawtx": "tcp://127.0.0.1:28335"
```

Query active endpoints (Core-compatible):

```powershell
dogego-cli getzmqnotifications
```

Same multipart wire format as Core (`command`, payload, 4-byte LE sequence). Blocks fire on `ConnectBlock`; txs on mempool admission.

## Workflow 7: Restart resume (DogeGo)

After stop/start during IBD, checkpoint and assist pool should remain healthy:

```powershell
.\scripts\core_restart_resume_check.ps1
```

Or bundled with Milestone E:

```powershell
$env:DOGEGO_IBD_SOAK = "1"
$env:DOGEGO_RESTART_RESUME = "1"
.\scripts\core_operator_workflow_cert.ps1
```

**Pass criteria:** `rawblocks_sync.json` probe height within ~64 of `dogego_contiguous_raw_height`; during deep body lag, `assist_peer_pool` > 0.

## Workflow 8: Maintenance / reindex (DogeGo)

Core-equivalent index and chain verification RPCs:

```powershell
.\scripts\core_maintenance_workflow.ps1
```

Or bundled:

```powershell
$env:DOGEGO_IBD_SOAK = "1"
$env:DOGEGO_MAINTENANCE_PROBE = "1"
.\scripts\core_operator_workflow_cert.ps1
```

**Pass criteria:** `verifychain 4 0` true when synced; `getindexinfo` reports tx index; `getchaintxstats` returns `window_tx_count`.

## Workflow 10: dogego-live scheduled CI (reboottestnet)

Cross-platform cert chain for self-hosted GitHub Actions runners (`dogego-live` label). Requires reboottestnet DogeGo on `:44556` and Core on `:44555` with wallets enabled.

**Offline prerequisites** (run on any dev machine before touching `dogego-live`):

```powershell
go run ./cmd/dogego cert offline
go run ./cmd/dogego cert wallet-import
go run ./cmd/dogego cert pq          # optional PQ format/carrier (~40s)
go run ./cmd/dogego cert operator   # optional deep Milestone E (~5-20 min)
```

Bundle: `scripts/cert_offline_prerequisites.{ps1,sh}` (same gates; optional PQ/operator flags).

`dogego cert wallet-import` is the superset offline gate (BIP39/BIP38, signer, UI/RPC import, and wallet.dat fixtures via `wallet/pool_replay.go`). Use **`dogego cert wallet-migration`** when you only need wallet.dat probe/import (including `-live-probe` / `-live-import` on dogego-live); offline-only: **`-offline-only`**. **`dogego cert operator`** adds consensus differential harness + field-evidence on top of wallet-import. **`dogego cert pq`** covers OP_RETURN commitment + TX_C/TX_R carrier format only (no production PQ safety claim).

**Readiness (light):**

```powershell
go run ./cmd/dogego cert provision -preflight -json
go run ./cmd/dogego cert setup-parity -mine-bootstrap
go run ./cmd/dogego cert weekly -mine-bootstrap -require-wallet-dat
```

**Full weekly bundle** (mirrors `ci_scheduled_weekly_live.ps1` - Core 24/24 + corruption mini):

```powershell
go run ./cmd/dogego cert enable-weekly -require-wallet-dat
go run ./cmd/dogego cert weekly-live -mine-bootstrap -require-wallet-dat
```

**One-shot orchestrator** (same stages; optional GitHub vars + Milestone B soak):

```powershell
go run ./cmd/dogego cert workflow10 -mine-bootstrap -require-wallet-dat
go run ./cmd/dogego cert workflow10 -enable-github -github-apply -mine-bootstrap -require-wallet-dat -include-live-soak
go run ./cmd/dogego cert workflow10 -skip-scripts -mine-bootstrap -require-wallet-dat -json
```

**Preflight-only** (skip PowerShell Core gate scripts; useful while provisioning):

```powershell
go run ./cmd/dogego cert weekly-live -skip-scripts -mine-bootstrap -require-wallet-dat -json
```

**Milestone B multi-hour soak** (mirrors `ci_milestone_b_full_gate.ps1`; disruptive):

```powershell
go run ./cmd/dogego cert live-soak -duration-min 60 -require-soak-env
go run ./cmd/dogego cert live-soak -skip-scripts   # doc/ui + preflight only
```

**GitHub Actions dispatch:**

```powershell
gh workflow run dogego.yml -f live_weekly=true -f require_wallet_dat=true
gh workflow run dogego.yml -f live_soak=true
```

**Pass criteria:** weekly-live exits 0; Core-aligned gate reports 24/24 stateful rows; corruption mini completes; live-soak ends with `verifychain 4 0` after timed inject cycles; reboottestnet E2E runbook includes **BIP152 HB** probe (`core_bip152_probe.ps1` / `GET /api/core-bip152-probe`).

**Web UI:** Features → **Workflow 10 preflight** runs `GET /api/core-workflow10-probe` (mirrors `dogego cert workflow10 -skip-scripts`).

See [STANDALONE_FULLNODE_ACCEPTANCE.md](STANDALONE_FULLNODE_ACCEPTANCE.md) and [ROADMAP.md](../ROADMAP.md).
