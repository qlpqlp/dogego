# DogeGo operator runbook (mainnet + reboot testnet)

Practical guide for running DogeGo as a **full node** when you might otherwise use **Dogecoin Core**. DogeGo is **not** Core - read [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md) first.

**Protocol lock:** mainnet block, header, script, subsidy, and auxpow rules follow Dogecoin Core. DogeGo does **not** introduce protocol forks or new consensus activations on mainnet. See [ROADMAP.md](../ROADMAP.md) **Dogecoin protocol lock**.

## Scripts and OS (Windows, Linux, macOS)

Prefer these in order:

1. **`dogego cert …`** (Go CLI) - works the same on every OS. See `dogego cert --help`.
2. **Web UI** - Features probes, Sync dock, Console (`http://localhost:2013/`).
3. **HTTP** - `curl` (Linux/macOS/Windows) or any JSON-RPC client against `rpc` listen.
4. **`scripts/*.sh`** - Linux/macOS twins when present.
5. **`scripts/*.ps1`** - Windows PowerShell (or `pwsh`); also used by CI / `dogego-live`.

Many IBD helpers (`watch_sync`, `node_health`, `log_ibd_progress`, …) are still PowerShell-first. On Linux/macOS use the dashboard, `curl`, and `dogego cert ibd-convergence` / `dogego cert operational` instead. Full map: [scripts/README.md](../scripts/README.md).

## When to use DogeGo vs Core

| Use DogeGo when… | Use Core when… |
|------------------|----------------|
| You want a Go codebase, embedded web UI, and documented native storage | You need maximum network compatibility and battle-tested release binaries |
| Reboot **testnet** development / CI | You mine mainnet at scale with the full pool stack |
| You want to **test DogeGo** (beta) and help tune it | You need native USB/HID signers without HWI (out of scope; use HWI `signer_cmd`) |

## Networks

| Config `network` | Chain | Notes |
|------------------|-------|--------|
| `mainnet` | Legacy Dogecoin mainnet | Scrypt PoW + auxpow era; hardest sync |
| `testnet` | Reboot testnet in this repo | `mine=true` solo founder; real scrypt PoW (`RelaxedPoW=false`); Digishield min-diff |

Datadir layout: `<datadir>/mainnet/` or `<datadir>/testnet/` with `headers/` (segment files), optional legacy `headers.bin.legacy` after migrate, `headers_sync.json`, `headers_aux.bin`, `rawblocks/`, `indexes/tx/`, `wallet.json`.

**One node per network datadir:** DogeGo holds an exclusive lock on `<datadir>/<network>/.dogego-process.lock` (Core `.lock` analogue). Starting a second `dogego node` against the same chain folder fails fast - stop the other process first (Task Manager / `Get-Process dogego` on Windows; `pgrep -a dogego` or your service manager on Linux/macOS).

On first start after upgrade, DogeGo **migrates** monolithic `headers.bin` into `headers/seg/NNNNNNNNNN.bin` (2000 headers per segment) + `headers/manifest.json`, then renames the old file to `headers.bin.legacy`. Crash recovery truncates torn segment tails and realigns `headers_sync.json` on open - no manual step required.

`headers_aux.bin` is padded to match `headers.bin` after header rewind or each sync batch (empty records until auxpow backfill from `rawblocks/`). You do not need to delete it when bodies lag headers. Auxpow slots are filled from stored `rawblocks/` as bodies arrive: immediately at the **body frontier** and around height **371337**, otherwise via bounded batch backfill (`aux backfill … through N` in logs). This avoids rewriting a multi-million-line `headers_aux.bin` on every block during deep IBD.

## Standalone certification (automated)

DogeGo includes offline certification commands (no P2P, no Core required). Same on Windows, Linux, and macOS:

```bash
cd DogeGo
go run ./cmd/dogego cert offline          # CI push/PR gate (~4 min)
go run ./cmd/dogego cert wallet-import     # BIP39/BIP38 + signer + wallet.dat
go run ./cmd/dogego cert operator          # Milestone E deep cert (~5-20 min)
```

Bundle (ROADMAP offline prerequisites): `scripts/cert_offline_prerequisites.sh` (Linux/macOS) or `scripts/cert_offline_prerequisites.ps1` (Windows). Offline gate twins: `ci_offline_gate.sh` / `ci_offline_gate.ps1`.

Focused slices:

```bash
go run ./cmd/dogego cert field-evidence    # Milestone A mainnet field corpus
go run ./cmd/dogego cert wallet-migration  # wallet.dat fixtures + live probe/import
go test ./node -run TestOperatorWorkflowStandaloneCertification -count=1
```

The operator cert runs consensus differential harnesses, UTXO/journal crash repair, node recovery, field-evidence, and wallet import tests. It does not replace mainnet IBD soak tests.

Cross-platform offline gates:

```bash
go run ./cmd/dogego cert offline
go run ./cmd/dogego cert field-evidence
go run ./cmd/dogego cert wallet-import
go run ./cmd/dogego cert wallet-migration
```

`dogego cert wallet-migration` exercises synthetic Core `wallet.dat` fixtures (plain + encrypted BDB), RPC `dogego_probewalletdat` / `dogego_importwalletdat` (pool metadata, **`pool_indices_replayed`** on HD import via `wallet/pool_replay.go`), and optional `-live-probe` / `-live-import` when a node is running. See [OPERATOR.md](OPERATOR.md) § Core wallet.dat keypool and [WALLET.md](WALLET.md).

Optional live mainnet probes (node must be running):

```bash
# Prefer Features → probes, or:
curl -sS "http://localhost:2013/api/core-operator-cert?refresh=1"
# Windows helper: DOGEGO_IBD_SOAK=1 scripts/ibd_soak_cert.ps1
```

This checks `getblockchaininfo` fields such as `headers`, `blocks`, `dogego_contiguous_raw_height`, and `dogego_genesis_missing`.

Full Core operator bundle (offline + live + optional side-by-side):

```bash
go run ./cmd/dogego cert offline
go run ./cmd/dogego cert milestones-bde
# Live Windows/CI helper (optional): scripts/core_operator_workflow_cert.ps1
#   with DOGEGO_IBD_SOAK / DOGEGO_IBD_CONVERGE / DOGEGO_CORRUPTION_SOAK as needed
```

Single live end-to-end probe (health + restart-resume + maintenance + reindex + BIP152 HB + optional wallet):

```bash
curl -sS "http://localhost:2013/api/core-end-to-end-probe"
# Windows/CI helper: scripts/core_end_to_end_workflow.ps1
# Optional Core compare: DOGEGO_CORE_COMPARE=1 with that script
```

BIP152 HB only:

```bash
curl -sS "http://localhost:2013/api/core-bip152-probe"
# Windows helper: scripts/core_bip152_probe.ps1
```

Long-haul CSV logging during soak (Windows helper today): `scripts/log_ibd_progress.ps1 -OutFile ibd_progress.csv -IntervalSec 60`. Cross-platform alternative: Overview sync dock + `dogego cert ibd-convergence`.

## Web UI Core parity probes (loopback)

When the dashboard is open on `127.0.0.1`, DogeGo exposes the same live gates as the operator cert scripts **without** requiring PowerShell or bash:

| UI location | API / action |
|-------------|----------------|
| **Features → Run all probes** | `GET /api/core-operator-cert?refresh=1` (90s cache; use `?refresh=1` to bypass) |
| **Features → Certification** | Live **14** web gates (incl. Milestone D setup-parity, IBD convergence snapshot) + script-only soak matrix; **View probe** on each row scrolls to the matching probe card (`#features/feat-core-compare`, … `#features/feat-core-bip152`, `#features/feat-core-runner`) |
| **Overview → Network** | Operator cert pill from `/api/summary` (`dogego_operator_cert_*`) plus mempool corpus/parity (`dogego_mempool_*`) when cache is warm; click failed gate summary to jump to probe card |
| **Settings → Advanced** | Core RPC (`core_rpc_addr`) + **Test Core connection** (`POST /api/core-test`); cached operator cert `N/M` + mempool corpus/parity from `GET /api/core-status` |
| **Console** | One-click probe buttons (`/api/core-compare`, `/api/mempool/parity-probe`, `/api/core-end-to-end-probe`, …) |
| **Sync dock (IBD)** | Operator cert `N/M` from cached probes |

Lightweight status without re-running probes: `GET /api/core-status`.

On dashboard startup, DogeGo **warms the probe cache** after ~8s (when RPC dispatch is ready) so `/api/summary` can show `dogego_operator_cert_*` and `dogego_mempool_*` without clicking **Run all probes**.

During **UTXO snapshot body replay**, `getblockchaininfo` and `/api/summary` expose `dogego_utxo_bodies_aligned`, `dogego_utxo_body_replay_remaining`, and `dogego_snapshot_body_replay_pct` - the web UI shows these on Overview → Sync and the sync dock (Core does not surface this operator signal).

Configure Core on loopback (`127.0.0.1:22555` mainnet) for side-by-side compare and mempool parity rows. See [WEB_UI.md](WEB_UI.md) and [CORE_PARITY_GAPS.md](CORE_PARITY_GAPS.md).

### Milestone D - mempool policy corpus (58 templates)

Offline gate (no live node required):

```powershell
go test ./consensus -run TestEvalMempoolCorpus -count=1
# or
.\scripts\core_mempool_corpus_probe.ps1
```

Live loopback (node + web UI on `localhost:2013`):

```powershell
curl http://localhost:2013/api/mempool/parity-probe
curl "http://localhost:2013/api/mempool/parity-probe?corpus=full"
curl http://localhost:2013/api/mempool/stateful-status
```

| Check | Pass bar |
|-------|----------|
| **offline_corpus** | `offline_corpus.passed` == **58** (includes BIP125 rule 2/5 rows `rbf_too_many_conflicts`, `rbf_new_unconfirmed_input`) |
| **stateless live** | **32/32** `testmempoolaccept` rows (`mempool_parity_rpc.json`) via `GET /api/mempool/parity-probe` or `.\scripts\core_mempool_parity_probe.ps1 -DogeGoOnly` |
| **stateful offline** | **26/26** offline eval; **24/24** wallet-anchored live scenarios on reboot testnet (`mempool_stateful_parity_reboottestnet.ps1 -Scenario all`) |
| **E2E bundle** | `GET /api/core-end-to-end-probe` includes `offline_corpus`, `bip125_offline`, and `mempool_parity` steps |

Core side-by-side (optional): set `core_rpc_addr` in Settings → Advanced, then `DOGEGO_MEMPOOL_PROBE=1` in `core_operator_workflow_cert.ps1` or `.\scripts\core_mempool_parity_probe.ps1` without `-DogeGoOnly`.

BIP125 rule 2/5 offline-only rows (no wallet-anchored live scenario):

```powershell
.\scripts\core_mempool_bip125_offline_probe.ps1
```

After **Run all probes**, `/api/summary` exposes `dogego_mempool_offline_corpus_passed/total` and `dogego_mempool_parity_passed/total` when the probe cache is warm.

## Core wallet.dat migration

Migrating from Core’s Berkeley DB `wallet.dat` into DogeGo’s `wallet.json`:

1. **Probe** (no import): `dogego_probewalletdat /path/to/wallet.dat` - or Receive tab / `GET /api/core-wallet-probe` when **`DOGEGO_WALLET_DAT`** is set.
2. **Import**: `dogego_importwalletdat` (native BDB first; encrypted via `options.passphrase`; Core `dumpwallet` fallback when `core_rpc_addr` is set).
3. **Cert**: `go run ./cmd/dogego cert wallet-import` (offline superset) or `go run ./cmd/dogego cert wallet-migration` (wallet.dat fixtures + live probe/import); dogego-live weekly can require a real fixture with `-require-wallet-dat`.

| Field | Operator meaning |
|-------|------------------|
| `pool_keys_matched` | Pool pubkey also has a spend key in wallet.dat |
| `pool_keys_unmatched` | **Pool-only** row - no spend key in BDB; DogeGo cannot recover the private key |
| `pool_unmatched_entries` / `pool_unmatched_hint` | Pool-only rows and operator guidance shown on probe/import |
| `pool_indices_replayed` | `true` after import when matched HD receive pubkeys replay into `hd_keypool` (`wallet/pool_replay.go`) |
| `pool_core_indices_stored` / `hd_keypool_core_index` | Core pool index numbers stored for matched HD receive keys |
| `keypool_refill_size` | HD `keypoolrefill` size used when pool-only rows remain |
| `keypool_hint` | Guidance when Core keypool entries are present - run **`keypoolrefill`** after migration |
| `address_book_keypool_count` / `address_book_core_pool_indices_stored` | Live wallet probe (`GET /api/core-wallet-probe`, `scripts/core_wallet_workflow.ps1`) counts from `dogego_listwalletaddresses` |

Full detail: [OPERATOR.md](OPERATOR.md) § Core wallet.dat keypool, [CORE_SIDE_BY_SIDE_WORKFLOWS.md](CORE_SIDE_BY_SIDE_WORKFLOWS.md) wallet migration workflow.

## First start checklist

1. Run `dogego node` (setup wizard if no datadir) or use existing `dogecoinconf.json`.
2. Prefer **`node_mode=full`** and **tx index on** (`no_tx_index` false) for explorer and wallet accuracy.
3. Keep **`webui`** on `127.0.0.1` only unless you terminate TLS at a reverse proxy.
4. **Wallet:** enabled by default unless `-nowallet`. Use `encryptwallet` before mainnet funds.
5. **RPC:** optional for external apps; web **Console** uses in-process RPC on full nodes even when HTTP `rpc` listen is empty.

## Mainnet initial block download (IBD)

### What “sync %” means

- **`blocks`** / **`getblockcount`** = **chainActive** (last height validated through `ConnectBlock` / UTXO tip), not the header journal tip.
- **`verificationprogress`** during IBD ≈ **chainActive ÷ header tip** when headers are ahead of connect (stored bodies can be far ahead while connect catches up).
- **`dogego_body_verification_progress`** = contiguous **stored** bodies under `rawblocks/` ÷ header tip.
- **`dogego_headers_sync_progress`** = chainActive ÷ header tip when headers are ahead (header-chain catch-up vs connect).
- **`dogego_tx_verification_progress`** on `getblockchaininfo` approximates Core’s tx-based progress on mainnet when indexed (dashboard hero % prefers this during `initialblockdownload` on mainnet).
- Headers can run **ahead** of **connected** height - normal during IBD.

### Header sync stalls (`bad nBits`, height ~4080 / 6720 / retarget boundaries)

Usually **stale local `headers.bin` timestamps** from an interrupted sync, not a PoW formula bug.

**Automatic recovery (preferred):**

1. Let the node run - DogeGo auto-rewinds one retarget window and retries `getheaders`.
2. Watch logs for `header sync recovery: deep rewind` (one retarget window, not a 1-block loop).
3. During **forward block IBD**, if the header tip is **thousands of blocks ahead** of stored bodies, DogeGo **defers** automatic header truncates (`deferring header truncate … during forward block IBD`) so recovery does not chop a 500k header tip while bodies are still near height 8k.
4. Web **Overview → Recover header journal** or Console → `dogego_recoverheaders` (if the tip does not move, DogeGo still restarts background header sync when a prior failure is recorded).

**Manual recovery:**

- RPC: `dogego_recoverheaders` (no parameters).
- Web: `POST /api/chain/recover-headers` (loopback).
- Last resort: stop node, delete `mainnet/headers/` (and `headers_sync.json`, `headers_aux.bin` if present), restart (full header re-download). Legacy installs may still have `headers.bin` - deleting `headers/` + `headers.bin` is equivalent.

**Do not** delete header storage while the node is running.

### Header sync stuck near **~8%** (`aux parent chain id must be zero`)

**Symptom:** Dashboard header progress around **500k / 6.2M (~8%)** while **blocks** and **stored bodies** still move (e.g. height 3k-4k). Logs repeat:

```text
header N aux: aux parent chain id must be zero (litecoin merge-mining parent)
```

**Cause:** DogeGo builds **before** Core-parity auxpow parent-chain-id validation rejected valid mainnet headers (Core only rejects when the parent encodes Dogecoin chain id `0x62`).

**Fix:** rebuild and restart (no header wipe required):

```powershell
cd C:\Users\pvida\Documents\GitHub\dogecoin\DogeGo
go build -o dogego.exe .\cmd\dogego
```

Stop the old process, run the new binary. Header sync should pass **510k+** within minutes. Use **`.\scripts\watch_sync.ps1`** to confirm `headers` climbs while `blocks` catch up.

**RPC/UI signals:** `getblockchaininfo` sets **`dogego_post_aux_era_header_stall`: true** when catch-up is pending and the header tip is in the 509.5k-510.5k band; **`dogego_header_sync_recovery`** carries the operator hint. Overview shows the banner and **Recover header journal** when either is set.

**One-shot check after restart:** `.\scripts\check_header_progress.ps1` (exit 0 when `headers` ≥ 510000), or `.\scripts\upgrade_post_aux_verify.ps1` (parity flag + optional `-RequireHeadersPast510k` / `-WatchSec 120`).

**Binary capability flag:** `getblockchaininfo` → `dogego_auxpow_parent_chain_id_core_parity: true` on builds with Core-aligned aux parent chain ID validation (reject only parent encoding Dogecoin `0x62`).

**Do not** treat this as a corrupt `headers/` journal unless a **current** build still fails every peer with a different aux error.

### Headers far ahead of block bodies (normal mainnet IBD)

**Symptom:** `headers` or `headers_sync.json` tip is hundreds of thousands (e.g. **534k**) while `blocks`, `dogego_contiguous_raw_height`, or `rawblocks_sync.json` `next_probe_height` is still in the low thousands. Dashboard **header %** can look “stuck” while the node is healthy.

**Cause:** Header sync is much faster than downloading and verifying full blocks. This is expected after passing the post-aux era.

**Check:** Overview → Sync and Console `getblockchaininfo`, or Windows helpers `scripts/sync_status.ps1` / `scripts/node_health.ps1`. Combined monitor: `dogego cert ibd-convergence` or Windows `scripts/ibd_monitor.ps1`. Live tail: sync dock or `scripts/watch_sync.ps1`. Clean restart after upgrade: restart the binary (same datadir) or Windows `scripts/restart_node.ps1 -Rebuild`. Stale checkpoint ages (>2h) usually mean the node is stopped - start `dogego` again to resume body IBD.

**Speed up bodies (optional):** in `dogecoinconf.json` raise **`maxoutbound`** (up to 32) and **`block_sync_workers`** (up to 24, or 0 = auto from `maxoutbound`). Use **`p2p_connectivity=both`** and **`upnp=auto`** when you can accept inbound peers. See [OPERATOR.md](OPERATOR.md) § Performance tuning.

### Stored bodies ahead of chainActive (connect lag after restart)

**Symptom:** `dogego_contiguous_raw_height` or `rawblocks_sync.json` probe is thousands ahead of RPC **`blocks`** (chainActive). `node_health.ps1` warns `connect_lag_behind_stored_bodies`. Dashboard Sync tab may show **Connecting stored blocks**.

**Cause:** Block files download faster than UTXO connect replay, especially right after restart when `utxo.cache` reloads an older chainActive tip while `rawblocks/` already holds deep history.

**Check:** `getblockchaininfo` → `dogego_stored_bodies_ahead_connect` (or `dogego_connect_lag`), `dogego_connect_blocks_per_minute` (connect rate), `dogego_raw_sync.blocks_per_minute` (download rate), and `dogego_raw_blocks_ahead_of_contiguous` (stored holes ahead of contiguous frontier). `watch_sync.ps1` shows both rates.

**Expected:** A background connect catch-up worker runs during body IBD (100ms poll when lag is high). Logs show `connect catch-up: chainActive … (+N, lag …)` when replay advances in batches. **`dogego_connect_blocks_per_minute` > 0** and **`blocks`** climbing - leave the node running; avoid frequent restarts (each restart reloads `utxo.cache`).

**Performance notes (current builds):** Connect replay is serialized via `utxoConnectMu`. While the **primary P2P session is paused** (header recovery), block-assist keeps body IBD alive (`maybeEnsureBlockAssistDuringNoPrimary`). During deep body IBD with **`ibd_optimize`**, post-batch **inline connect** only runs when connect lag is extreme (routine connect stays on the catch-up worker so getdata is not starved). Auto **`dbcache`** sizes the UTXO flush budget from free RAM (`dogego_dbcache_mb`). Poll `dogego_raw_sync.assist_peer_pool` and `assist_active_sessions` on `getblockchaininfo`. BIP158 filter catch-up resumes after restart (`dogego_filter_index_through` / `dogego_filter_index_lag`).

**Not a stall:** Body download can pause while connect catches up; use `ibd_convergence_check.ps1` and confirm **`blocks`** (not just `contiguous_raw`) advances.

**Not a stall (body-only phase):** When **`blocks` == `contiguous_raw`** (connect lag 0), convergence checks pass while `in_flight_batches > 0` on active getdata. Pruned peers may return **undersized invalid payloads** (<140 B or failing consensus); current builds reject those, penalize the peer, and rotate primary/assist links automatically. **Note:** valid ancient coinbase-only blocks (e.g. height **10006** is **213 B**) must not be confused with pruned stubs - DogeGo uses a **140 B** floor for mainnet heights **< 500k** plus full block validation.

### Connect stuck at a fixed height (chainActive not advancing)

**Symptom:** Web activity log repeats `connect catch-up: utxo sync: connect stalled at height N (contiguous bodies through …)` and RPC **`blocks`** stays at **N** for many minutes. `node_health.ps1` reports **`connect_chain_active_stuck_at_N`**. `ibd_convergence_check.ps1` fails on **`blocks+`**.

**Automatic repair:** connect failures with `missing funding height` or `connect stalled at height` trigger **immediate** sparse/lag txindex repair (same pass retries connect). Periodic sweeps also run `RepairTxIndexIfSparse` and `RepairTxIndexIfLag`. Undersized block stubs penalize peers (`NoteStubBlock`) and rotate primary/assist links. Body-only IBD (connect lag 0) uses a **3 min** stall-recovery interval. Logs: `tx index sparse repair:` or `tx index repair:`.

**Common cause (fixed in current sources):** txindex missing a parent tx while the UTXO set still has the coin - ConnectBlock failed with `missing funding height` and chainActive froze (e.g. height **6856→6857**). Current builds use **UTXO confirmation height** when txindex lookup fails (`consensus/prevout_height.go`). Rebuild and restart if you still see only `connect stalled at height N` with no `connect height N+1:` line.

**Check:** Web UI → Activity (or `GET http://localhost:2013/api/logs?limit=200`) for `connect height N+1:` (consensus/UTXO error). Bodies through **N+1** should exist (`HasStoredBody`); if missing, see raw-body gap above.

**Recovery:**

1. **Nudge connect (non-blocking):** `.\scripts\nudge_connect.ps1` (wraps async `syncutxo`, default **8** blocks) or `Invoke-DogeGoJsonRpc -Method syncutxo -Params '[4]'`. Poll `getblockchaininfo` → **`blocks`**, `dogego_connect_blocks_per_minute`, `dogego_utxo_connect_in_flight`, `dogego_syncutxo_in_flight`. Do **not** use large values (e.g. 128) - each block can take seconds and long runs block the connect mutex.
2. **Checkpoint chainActive** before restart: `.\scripts\save_utxo_snapshot.ps1` (polls until `dogego_utxo_snapshot_height` catches up) or JSON-RPC `saveutxosnapshot` (async; check `dogego_utxo_snapshot_save_in_flight`).
3. **Rebuild** `dogego.exe` from current sources (connect errors are logged during IBD on recent builds) and restart: `.\scripts\restart_node.ps1 -Rebuild`.
4. If the same height persists, stop the node and run offline diag:  
   `go test -tags datadir_diag ./node -run TestDatadirConnectFromSnapshot -v -timeout 30m`  
   (adjust height in test if needed) to print the ConnectBlock failure.
5. Last resort (destructive): `truncatetoheight <N>` via RPC/console, or delete `utxo.cache` / replay chainstate per [OPERATOR.md](OPERATOR.md), then restart IBD connect from **N**.

### Long-haul mainnet soak (multi-day body IBD)

Leave **`dogego.exe`** running. Log progress and review weekly:

```powershell
# Continuous CSV (dedicated terminal)
.\scripts\log_ibd_progress.ps1 -OutFile ibd_progress.csv -IntervalSec 60

# Or Task Scheduler hourly
.\scripts\ibd_snapshot.ps1 -OutFile ibd_progress.csv

# Weekly summary
.\scripts\ibd_progress_report.ps1 -Csv ibd_progress.csv -LastRows 10080

# On-demand gate (health + 2-minute forward progress)
.\scripts\ibd_monitor.ps1 -ConvergeSec 120

# JSON for Task Scheduler / monitoring (includes body_ibd ETA snapshot)
.\scripts\ibd_monitor.ps1 -Json -ConvergeSec 120
```

**Body IBD pump (current builds):** proactive `getdata` every **~1.5s** on primary, relay, and block-assist lanes (not only P2P read idle). Stall recovery runs whenever bodies lag headers; body-only phase uses a **90s** refresh window when connect has caught up. Watch logs for `progressive getdata heights …` every 1-2s and `Body IBD pump:` lines from `ibd_monitor.ps1`.

**Header sync resume:** while bodies lag headers by **>50k** (headers at **534k**, bodies below **~484k**), getheaders is paused (`dogego_body_ibd_header_paused=true`). When contiguous bodies enter that window, DogeGo logs `body IBD pause lifted … resuming header catch-up` and background header sync continues toward the network tip (~6.2M).

Certification bundle (offline tests + live probes):

```powershell
$env:DOGEGO_IBD_SOAK = "1"
$env:DOGEGO_IBD_CONVERGE = "1"
.\scripts\core_operator_workflow_cert.ps1
```

### AuxPoW boundary (mainnet height **371337**)

The first merge-mined block must carry an auxpow header (not legacy scrypt). During IBD you may see a log like `legacy scrypt header after auxpow activation` at height 371337 - DogeGo treats that as a **bad peer** and tries the next candidate (same as `bad nBits`). Core **checkpoint** hash at **371337** is enforced when `-checkpoints` is on (default); a **checkpoint hash mismatch** in local `headers.bin` triggers an automatic rewind to height **371336** and retry (same as auxpow journal recovery).

Older DogeGo builds also logged `aux hash block mismatch` at the same boundary; current builds follow Core and do **not** reject headers solely for a stale wire `hashBlock` field (merkle + coinbase script are checked). If every peer still fails, run **`dogego_recoverheaders`** or delete `mainnet/headers.bin` + `headers_aux.bin` only when you suspect a wrong-network or corrupted journal (node stopped).

If you set a fixed **`peer`** in config and it is **unreachable**, handshake fails, or drops mid-sync, DogeGo **falls back to DNS/fixed seeds** instead of exiting. If the **primary** session is lost repeatedly during IBD (Windows `wsasend`, resets, redial exhaustion), the node **stays up**: block-assist keeps downloading bodies while **background header recovery** runs (DNS/fixed-seed discovery is refreshed immediately on pause, not only on the 5-minute poll).

If **every** header peer drops with a network error (`wsasend`, timeout, reset), the node **stays running** and retries header sync in the background (web UI at `http://localhost:2013/` keeps working). **Block bodies still download** via block-assist workers while headers catch up. Check logs for `header sync paused … retrying in background` and `block-assist`.

After any header rewind (`dogego_recoverheaders`, auto bad-nBits recovery, `truncatetoheight`), DogeGo **resets the block download cursor** to the lowest missing height so body sync does not resume from a stale `rawblocks_sync.json` checkpoint. It also clears the **IBD exit latch** (so `initialblockdownload` reflects catch-up again), re-requests **mempool** after the next catch-up, and skips **parallel header assist** while bodies still lag headers (avoids conflicting header batches on one journal).

### `bad-cb-amount` at height 2 (`subsidy 55351800000000`, `out 72975200000000`)

That pair means the **binary is too old** - legacy subsidy used modulo instead of Core’s Boost `uniform_int` mapping. Current DogeGo refuses to start on mainnet if the RNG self-check fails, and logs a rebuild hint on this exact error during block download.

**Fix:** rebuild and restart:

```powershell
cd C:\Users\pvida\Documents\GitHub\dogecoin\DogeGo
go build -o dogego.exe .\cmd\dogego
```

Then stop the node and run the new `dogego.exe`. Height 2 should store without `bad-cb-amount`. Current builds also **connect blocks below height 4096 immediately** (not deferred during assume-valid IBD) and reject invalid coinbase amounts **before** writing `rawblocks/`.

### Block body IBD slow

**Biggest difference vs Core:** DogeGo’s default **tx index** writes **one file per transaction** under `indexes/tx/`. During deep body IBD, that used to run on every downloaded block and could dominate disk I/O (Core uses LevelDB and indexes on connect). Current builds **defer tx/addr/filter indexing on the download path** while bodies lag headers by more than ~512, and index on **ConnectBlock** / catch-up instead. Bundled `blk*.dat` layout also no longer **fsyncs every block**.

Still slow? Operator levers:

- Keep **`ibd_optimize`** on (default) and leave **`dbcache`** at **0** (auto) or raise it explicitly if you have spare RAM (see [OPERATOR.md](OPERATOR.md) § Sync speed).
- Increase **`maxoutbound`** / block-assist workers in config (see [OPERATOR.md](OPERATOR.md)).
- Prefer **`block_storage_layout=bundled`** (Core-style `blk*.dat`) over default per-file `rawblocks/*.bin` on mechanical disks.
- For maximum download speed only: **`no_tx_index=true`** (disables explorer tx/address search; rebuild later with `reindextx` after turning the index back on).
- Ensure disk space; check Overview **storage** line (**connected** chainActive vs **stored bodies** vs header tip).
- `getblockchaininfo` → `dogego_blocks_behind_headers`, `dogego_sync_eta`. Dashboard **blk/min** is a **recent-window** rate; lifetime mean is `dogego_blocks_per_minute_lifetime` (early bursts no longer dilute the live number after an hour).
- If `rawblocks_sync.json` checkpoint was far ahead of **bodies through**, DogeGo clamps it on startup and logs `checkpoint height … ahead of contiguous …; forward sync from …`. The checkpoint may legally be height **0** when genesis is still missing (forward sync always includes height 0 when needed).
- After **5 minutes** without new stored blocks near the frontier (early IBD, bodies through **< 1000**), the node refreshes peers and **DNS/fixed-seed discovery** (`IBD stall` log). During **deep body IBD** (stored through **≥ 1000** but headers still far ahead), that window is **10 minutes**; otherwise **15 minutes**. While **`dogego_genesis_missing`** is true, that window is **2 minutes** (pruned peers cannot serve height 0; archival rotation must happen quickly).
- **Connect catch-up after restart:** stored bodies can be thousands ahead of **`blocks`** (chainActive). DogeGo runs a dedicated connect catch-up worker during body IBD (`connect catch-up: chainActive …` in logs). Watch **`dogego_connect_blocks_per_minute`** and **`dogego_stored_bodies_ahead_connect`** - not a stall if connect rate is > 0. Operator scripts: `node_health.ps1`, `ibd_monitor.ps1`.
- **Archival peer preference:** block-assist and primary redial deprioritize BIP159 `NODE_NETWORK_LIMITED` / pruned peers when fetching ancient heights (≤512 and genesis). Persisted in `block_peer_scores.json` v2 (NODE_* services + start height). If genesis getdata returns **notfound**, the primary session is torn down and redialed to a full archival peer.
- **`EnsureLocalGenesis`:** when headers exist but height 0 is missing from `rawblocks/`, DogeGo stores the Core-shaped genesis block from chainparams locally (startup sweep, IBD stall recovery, and progressive sync all call this). Startup also rejects a journal genesis header that does not match chainparams (`journal genesis header mismatch` → run header recovery or wipe `headers/`).
- **`native_raw_block_count`** on `/api/summary` is reconciled from disk when it lags **`contiguous_raw_height`** (stale in-memory counter after crash or bulk copy). Orphan `.bin` files above the frontier can make the count slightly higher than contiguous+1 until the gap closes.
- **`chain_active_height`** can lag **`contiguous_raw_height` by thousands** during forward IBD: bodies store first, then UTXO `ConnectBlock` catches up in batches (watch `Connected block height N` / `UTXO cache advanced through height N` in logs).
- **`defer connect at height N`** in logs for large **N** while **connected** height is still low is normal: inv blocks are stored as orphans until heights in between are downloaded; only the contiguous frontier connects.
- **`connect height N pending`** - block body is already in `rawblocks/` (Core-style store before `ConnectBlock`); the node retries connect via the contiguous frontier. Not a failed download.
- **`dogego_genesis_missing`: true** means height **0** is not in `rawblocks/` yet; the node will request it via getdata (check logs for `progressive getdata heights 0..0`).
- On startup, DogeGo **removes undersized `rawblocks/*.bin` stubs** (e.g. 224-byte test files) and logs `removed N undersized raw block stub(s)` - then re-downloads like Core would not treat those as valid blocks.
- **Early mainnet bodies (heights 1-~10k and beyond)** are often **~190-250 B** coinbase-only blocks (height **10006** is **213 B** on Core). DogeGo counts them as stored when ≥**140 B** for heights **< 500k** (genesis still requires ≥**200 B**). A prior **280 B** floor at heights ≥ 10k incorrectly stalled IBD at **10006** - upgrade and restart if `contiguous_raw` sticks there with `too short … need >= 280` in logs.
- Progressive block download uses **16 blocks per getdata** per peer (Core `MAX_BLOCKS_IN_TRANSIT_PER_PEER`); the primary connection may issue up to **3** batches while waiting for P2P messages during IBD, plus **proactive pump ticks every ~1.5s** on primary/relay/assist (`node/ibd_body_pump.go`).
- Batched getdata uses a **Core-scaled block download timeout** (~5 minutes base, +~2.5 minutes per other parallel sync lane); see `dogego_raw_sync.block_download_timeout_sec` on `getblockchaininfo`. When a lane keeps heights **in-flight** past that window, DogeGo **disconnects** the peer (same as Core `nDownloadingSince` / `BLOCK_DOWNLOAD_TIMEOUT_*`; log `block download timeout`).
- Header catch-up on a **stale header tip** (>24h old) uses a longer stall window before rotating peers (Core `HEADERS_DOWNLOAD_TIMEOUT_*`). Silent header peers are **rotated** and **down-ranked** in the block peer scorer (same as Core disconnecting a stalling headers peer).
- **`getblockchaininfo`**: `dogego_last_block_stall_peer` / `dogego_last_block_download_timeout_peer` (and timestamps) surface recent IBD peer disconnect reasons; `dogego_max_blocks_in_transit_per_peer` and `dogego_lane_in_flight` show per-peer in-flight getdata counts during forward IBD. **`dogego_body_ibd_eta_minutes`** and **`body_ibd_eta`** on `node_health.ps1 -Json` estimate time to fill bodies through the local header tip at the current download rate.
- **addnode** entries are seeded into the addrbook **tried** table on startup (Core tried-first dial priority).
- **Block stalling:** when the contiguous frontier height stays **in-flight** without a delivered block past the stall window, DogeGo releases the claim (`block download stall` log). **Near tip:** **2s** (Core `BLOCK_STALLING_TIMEOUT`) → **disconnect** + hard scorer cooldown. **Deep body IBD:** **~15s** → **soft release** (peer kept; brief cooldown) so ancient getdata latency does not rotate the whole assist pool. Snapshot fields: `block_stalling_timeout_sec`, `block_stalling_timeout_body_ibd_sec`, `last_block_stall_peer`, `last_block_stall_at` on `dogego_raw_sync`.
- **`dogego_sync_health`**: `forward_ibd_active` = bodies storing; `forward_ibd_stalled` = no storage for 5-15 min at the frontier - node realigns `next_probe_height` and refreshes assist peers automatically. **`dogego_frontier_stalling_since`** (also on Overview when stalled) is the unix time the contiguous frontier height has been in-flight without a block.
- **`dogego_header_catch_up_pending`**: true while header sync retries in the background; **`headers_catching_up`** on `dogego_sync_health` means block-assist is still storing bodies (not a false “stalled” alarm). If headers retry but bodies stop, health becomes **`header_attention`**.
- **`dogego_block_assist_active`**: true when parallel block-download workers are running (IBD); pair with **`dogego_block_assist_connections`** on `getnetworkinfo` when P2P is active.
- If **DNS/peer probe fails** on startup but **`headers.bin` already has a chain**, the node **still starts** (web UI + block-assist) and retries peers in the background - you do not need to delete the datadir.
- During IBD, when **bodies lag headers**, DogeGo **defers pulling more headers** from a far-ahead peer (Core-style) instead of treating that session as a hard failure.
- While **`dogego_header_catch_up_pending`** is true (no primary peer), the node **re-discovers peers every ~5 minutes** and keeps block-assist workers active.

## Index maintenance

| Task | RPC | Web UI |
|------|-----|--------|
| Rebuild tx index from raw blocks | `reindextx` [clear] | Settings → Sync → Rebuild tx index |
| Rebuild BIP158 filters | `reindexblockfilters` | Settings → Sync → Rebuild block filters |
| Truncate chain (destructive) | `truncatetoheight <h>` | Console only |

Run maintenance while the node is up; mainnet rebuilds can take hours.

## Mining

| Network | Method |
|---------|--------|
| **Mainnet** (legacy heights) | `generatetoaddress` (CPU scrypt; very slow at real difficulty) |
| **Mainnet** (auxpow era) | `createauxblock` + pool merge-mining → `submitauxblock` |
| **Reboot testnet** | `mine=true` in config or `generatetoaddress` |

Web **Console** includes presets for `generatetoaddress`, `createauxblock`, `getblocktemplate`.

**Certification:** Features → **Mining / GBT / aux probe** (`GET /api/core-mining-probe`), `dogego cert mining`, or `scripts/core_mining_workflow.ps1` (optional Core GBT compare when tips align). Live operator-cert gate: **`mining`**.

## Wallet (mainnet + testnet)

- File: `wallet.json` per network (BIP44 HD, `encryptwallet`).
- Web: Send / Receive / History tabs.
- PQ commitments: Settings → **pq_commitments** flag; optional OP_RETURN on Send. Offline format/carrier cert: `go run ./cmd/dogego cert pq` (recognition only - **not** production PQ safety).

## Security

- Loopback bind for web UI and RPC.
- `rpcallowip`, cookie auth, optional TLS - see [SECURITY.md](SECURITY.md).
- Never commit `wallet.json` or `.cookie` files.

## Quick RPC health check

```bash
# With cookie auth (replace path and port)
curl -s --user "$(cat mainnet/.cookie | tr ':' ' ')" \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"1.0","id":1,"method":"getblockchaininfo","params":[]}' \
  http://127.0.0.1:22555/
```

Inspect: `initialblockdownload`, `verificationprogress`, `dogego_contiguous_raw_height`, `dogego_genesis_missing`, `dogego_sync_status`, `dogego_sync_health` (`forward_ibd_active` = OK during catch-up; `forward_ibd_stalled` = no blocks stored recently), `dogego_sync_ok`, `dogego_header_sync_recovery`, `warnings`.

### Side-by-side with Dogecoin Core (same machine)

When both nodes run on mainnet, these fields should track similarly during healthy IBD (DogeGo adds `dogego_*` extensions):

| Field | Core | DogeGo |
|-------|------|--------|
| Header tip height | `getblockchaininfo.headers` | same |
| Connected chain tip | `getblockchaininfo.blocks` | same (chainActive, not raw store count) |
| IBD flag | `initialblockdownload` | same semantics (`-maxtipage`, min chain work) |
| Progress | `verificationprogress` | same + `dogego_body_verification_progress` |
| Genesis body | implicit (blocks ≥ 0) | `dogego_genesis_missing` when height 0 not in `rawblocks/` |

DogeGo stores bodies under `rawblocks/` before connect catches up, so **`blocks` can lag stored bodies** - use `dogego_contiguous_raw_height` on Overview or RPC to see download progress.

## Further reading

- [OPERATOR.md](OPERATOR.md) - config reference
- [RPC.md](RPC.md) - workflow examples
- [WALLET.md](WALLET.md) - wallet RPC subset
- [ROADMAP.md](../ROADMAP.md) - parity checklist
