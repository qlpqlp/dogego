# Connecting external applications to DogeGo

DogeGo exposes a **Dogecoin Core-compatible JSON-RPC** HTTP API. Most integrations use the same patterns as Core: HTTP POST, JSON-RPC 1.0, optional cookie or Basic auth.

## Endpoints

| Service | Default (configurable) | Protocol |
|---------|------------------------|----------|
| JSON-RPC | `127.0.0.1:22555` (mainnet-style port in `dogecoinconf.json`) | HTTP POST |
| Web dashboard | `localhost:2013` | HTTP GET (UI + `/api/*`) |

Bind both to **loopback** unless you deliberately expose them behind a TLS reverse proxy. See [OPERATOR.md](OPERATOR.md) and [SECURITY.md](SECURITY.md).

## Authentication

1. **Cookie file** (recommended for local tools): set `"rpc_cookie": true` in `dogecoinconf.json`. DogeGo writes `.cookie` under the chain datadir; send header `Authorization: Basic <user>:<password>` where user/password match the cookie file (same idea as Core).
2. **Fixed credentials:** `"rpc_user"` + `"rpc_password"` in config.
3. **Allowlist:** `"rpcallowip"` for non-loopback clients; optional `"rpcwhitelist"` for method allowlists.
4. **Rate limits:** `"rpclimit"` (requests/minute per IP); `"rpcauthmaxfail"` for failed auth.

## JSON-RPC request shape

```http
POST / HTTP/1.1
Host: 127.0.0.1:22555
Content-Type: application/json
Authorization: Basic ...

{"jsonrpc":"1.0","id":"1","method":"getblockchaininfo","params":[]}
```

**Batch:** send a JSON **array** of request objects; receive an array of responses (`rpc/server.go`).

## Example: `curl`

```bash
# Replace USER:PASS with cookie or rpc_user/rpc_password
curl -s --user 'USER:PASS' \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"1.0","id":1,"method":"getblockchaininfo","params":[]}' \
  http://127.0.0.1:22555/
```

## Example: Python

```python
import json
import urllib.request
import base64

url = "http://127.0.0.1:22555/"
auth = base64.b64encode(b"rpcuser:rpcpass").decode()
body = json.dumps({"jsonrpc": "1.0", "id": 1, "method": "getmempoolinfo", "params": []}).encode()

req = urllib.request.Request(url, data=body, method="POST")
req.add_header("Content-Type", "application/json")
req.add_header("Authorization", "Basic " + auth)
with urllib.request.urlopen(req) as resp:
    print(json.load(resp))
```

## Example: Go

Use any HTTP client; method list is in `dogego/rpc/dispatch.go` (`SupportedMethods`). Help strings: `rpc.MethodHelp(name)`.

DogeGo is suitable for **reboot testnet**, mainnet testing, tooling, and experiments. Please try it and report what needs tuning.

## Web dashboard APIs (not JSON-RPC)

Integrators building a **local control panel** can poll loopback REST-style endpoints (same origin as the dashboard):

| Endpoint | Purpose |
|----------|---------|
| `GET /api/summary` | Sync, network, wallet balance strip; **`wallet_index_height`**, **`needs_rescan`**, **`wallet_scan_index_ok`**, **`wallet_history_fast_path`**, **`wallet_listtransactions_utxo_walk`**, **`wallet_listtransactions_scan_pending`**, **`wallet_history_deferred`**, **`wallet_history_defer_reason`** (`ibd_active`, `connect_lag`, `scan_building`), **`scanning`** when wallet wired; **`addrbook_*`** tried/new bucket stats when P2P wired; History tab defers heavy **`listtransactions`** while scan builds index (>64 UTXOs); **`bip152_hb_to`** / **`bip152_hb_from`** / **`bip152_hb_max`** when P2P is wired; **`dogego_mempool_offline_corpus_*`** + **`dogego_mempool_parity_*`** when probe cache warm |
| `GET /api/p2p` | P2P connectivity snapshot (mode, peers, UPnP, IBD lanes, **`addrbook_*`** bucket stats, **`addrman_info`** Core-shaped summary, BIP152 HB counts) |
| `GET /api/wallet` | Wallet balance + immature mining + UTXO count (UTXO-cache fast path when node wired); **`keypool_size`**, **`pool_core_indices_stored`**, **`hd_keypool_core_index`**, **`wallet_index_height`**, **`chain_active_height`**, **`needs_rescan`**, **`wallet_scan_index_ok`**, **`wallet_history_fast_path`**, **`wallet_listtransactions_utxo_walk`**, **`wallet_listtransactions_scan_pending`**, **`wallet_history_deferred`**, **`wallet_history_defer_reason`**, **`rescan_from_height`**, **`scanning`**, **`signer_cmd_configured`** when disk/RPC wired; dashboard auto-starts incremental rescan once per session when caught up and (**`needs_rescan`** or **`wallet_listtransactions_utxo_walk`** with many UTXOs); reloads History when scan index becomes ready |
| `POST /api/wallet/rescan` | Background wallet block rescan (default from index+1; `{ "full": true }` from genesis; optional `"start_height"`) - backfills **`fee_koinu`** and tx hex in **`wallet.db`** |
| `GET /api/wallet/txs` | Paginated wallet history (`limit`, `offset`, `q`, `kind`); returns **`deferred`** + **`defer_reason`** (`ibd_active`, `connect_lag`, `scan_building`) instead of walking UTXOs during heavy sync/rescan when >64 UTXOs |
| `GET /api/wallet/utxos` | UTXO list for Send coin control (all wallet SpendScripts; UTXO-cache fast path) |
| `GET /api/wallet/addresses` | Address book rows (`dogego_listwalletaddresses`; sorted by HD path/type; **`iskeypool`** and **`hd_keypool_core_index`** when stored) |
| `POST /api/wallet/address/new` | New receive address (Core `getnewaddress`; loopback; wallet unlock if PIN enabled) |
| `POST /api/wallet/address/label` | Set address label (Core `setlabel`; loopback; wallet unlock if PIN enabled) |
| `POST /api/wallet/send` | Send DOGE; optional **`pq_mode: "carrier"`** when **`pq_carrier_enabled`** (TX_C/TX_R via `dogego_sendpqcarrier`); fee-related errors include `fee_hint` + `suggested_fee_rate` |
| `GET/POST /api/wallet/flags` | Read/toggle **`pq_commitments_enabled`** and **`pq_carrier_enabled`** (Settings → Wallet; mirrors `setwalletflag`) |
| `POST /api/wallet/import` | Import mnemonic, BIP38, or Core `wallet.dat` (`type`: `mnemonic` \| `bip38` \| `walletdat`; wallet.dat accepts `path`, optional `passphrase`, optional `via_core_rpc`; native import may return pool metadata + **`keypool_hint`** + **`pool_unmatched_hint`** + **`keypool_refill_size`** + **`pool_indices_replayed`** when HD replay succeeds) |
| `POST /api/wallet/keypool-refill` | Core **`keypoolrefill`** for HD wallets (optional `{ "new_size": 100 }`; loopback; returns updated **`keypool_size`** / **`change_keypool_size`** when available) |
| `POST /api/signer-test` | Quick HWI **`enumeratesigners`** from Settings → Advanced external signer form (loopback only; tests `signer_cmd` before save/restart) |
| `GET /api/core-wallet-probe` | Milestone E wallet workflow; **`getwalletinfo`** may include **`hd_keypool_core_index`** / **`pool_core_indices_stored`** / **`signer_cmd_configured`** after Core import; **`wallet_index_height`** / **`needs_rescan`** / **`wallet_scan_index_ok`** (indexed through tip - listtransactions uses wallet.db history); **`wallet_history_fast_path`** when receive rows exist but index lags tip (listtransactions skips UTXO receive walk; rescan may still backfill fee/hex); **`wallet_listtransactions_utxo_walk`** when no receive rows yet (listtransactions walks UTXO cache; rescan recommended for solo miners with many coinbases); **`wallet_listtransactions_scan_pending`** while rescan runs before first receive rows; **`wallet_history_defer_reason`** (`ibd_active`, `connect_lag`, `scan_building`) when History would defer - skips **`listtransactions`** latency gate (same rules as **`GET /api/wallet/txs`**); **`address_book_keypool_count`** / **`address_book_core_pool_indices_stored`** from `dogego_listwalletaddresses`; **`validateaddress`/`getaddressinfo` `iskeypool` round-trip** on first keypool row; **`pool_core_indices_stored` vs address book count** when both present; **`keypool_topup_ok`** after `getnewaddress` on HD wallets; **`wallet_tx_hex_ok`** / **`wallet_tx_fee_ok`** on first confirmed send via `listtransactions` → `gettransaction`; **`wallet_listtransactions_ok`** / **`wallet_listtransactions_ms`** (40 rows, <3s gate when not deferred); **`wallet_pq_send_ok`** / **`wallet_pq_tag`** when **`pq_commitments`** enabled and a confirmed send hex carries FLC1/DIL2/RCG4 OP_RETURN; optional **`psbt_roundtrip_ok`** / **`psbt_bip32_deriv_ok`** / **`hardware_psbt_hint`** (`walletcreatefundedpsbt` + `walletprocesspsbt` when unlocked with mature balance); when **`DOGEGO_WALLET_DAT`** is set, includes **`dogego_probewalletdat`** pool metadata (`pool_keys_matched`/`pool_keys_unmatched`, **`pool_unmatched_hint`**, **`pool_indices_replayed`** on probe is always false, **`pool_replay_scan_cap`** when **`pool_count`>0); mirrored in **`scripts/core_wallet_workflow.ps1`** |
| `GET /api/wallet/probe-walletdat?path=…` | Dry-run Core `wallet.dat` probe (`is_bdb`, `encrypted`, `encrypted_keys`, `pool_count`, `pool_pubkeys`, `pool_entries` with `spends_key_matched`, `pool_keys_matched`, `pool_keys_unmatched`, `pool_unmatched_entries`, `pool_unmatched_hint`, `pool_index_min`/`pool_index_max`, `pool_indices_replayed`, `needs_passphrase`, `can_import`, `hint`; plus **`hd_keypool_core_index`** / **`pool_core_indices_stored`** when built-in wallet has stored indices) |
| `GET /api/mempool` | Pool stats + relay policy |
| `GET /api/mempool/parity-probe` | Stateless testmempoolaccept rows (32) + `offline_corpus` (58 templates) + `stateful_live` Milestone D summary (incl. **`setup_parity_ok`** / balances from **`GET /api/core-setup-parity`** on reboot testnet) |
| `GET /api/mempool/stateful-status` | Read-only **`offline_corpus`** (58) + stateful offline corpus (26; 24 live-mapped) + live 24/24 gate hints |
| `GET /api/core-setup-parity` | Reboottestnet Milestone D setup parity check |
| `GET /api/capabilities` | Feature flags + full RPC catalog |
| `GET /api/docs` | Documentation hub (dashboard concepts + integration); includes former Guide topics |
| `GET /api/guide` | Legacy endpoint - same guide sections as `/api/docs` (Guide tab removed) |
| `GET /api/wallet/txs.csv` | Wallet transaction history CSV (loopback; wallet unlock may be required); **503** with **`X-DogeGo-Wallet-Defer-Reason`** when history deferred (same rules as `/api/wallet/txs`) |
| `GET /api/autostart` | OS login autostart registration vs config |
| `GET /api/update/status` | GitHub release check snapshot (`dogego_update_*` fields also on `/api/summary`) |
| `POST /api/update/check` | Force immediate GitHub release check (loopback) |
| `POST /api/update/download` | Download + verify release binary to `<datadir>/updates/` (loopback) |
| `POST /api/update/apply` | Install verified update and restart (loopback) |
| `POST /api/update/dismiss` | Hide update notice until a newer release |
| `GET /api/core-autostart-probe` | Operator cert autostart gate (same as `dogego cert autostart`) |
| `GET /api/core-founder-probe` | Reboot testnet founder preflight (`dogego cert founder`; skipped on mainnet) |
| `GET /api/core-runner-probes` | dogego-live runner readiness (`dogego cert weekly`; optional `?require_core=1` / `?require_wallet_dat=1`; `cli_weekly_live` / `cli_live_soak` hints; includes `preflight.wallet_dat_import` on live runners) |
| `GET /api/core-bip152-probe` | BIP152 v1: `getpeerinfo` `bip152_hb_to` / `bip152_hb_from` schema + HB negotiation when caught up; `getblockchaininfo` **`dogego_cmpct_*`** relay counters + **`cmpct_relay_schema_ok`** when caught up with peers; optional Core `getpeerinfo` when `core_rpc_addr` set |
| `GET /api/core-ibd-convergence-probe` | IBD progress snapshot (headers/blocks/contiguous, connect boost, body coverage); mirrors `ibdconvergence.CollectSnapshot`; timed window remains `dogego cert ibd-convergence` / `scripts/ibd_convergence_check.ps1` |
| `GET /api/core-addrman-probe` | Partial Core addrman snapshot (`getaddrmaninfo` + `getblockchaininfo` **`dogego_addrbook_*`** cross-check); mirrors `scripts/core_addrman_workflow.ps1` |
| `GET /api/core-pq-probe` | PQ format/carrier offline probe (FLC1/DIL2/RCG4 OP_RETURN + TX_C/TX_R round-trip; verifier-side only; bundled in **`GET /api/core-probes`**) |

These require a **loopback client** (browser on the same machine). Do not expose the dashboard to the public internet without a proxy and auth.

```bash
# Wallet history CSV (default webui 127.0.0.1:2013)
curl -s -o dogego-wallet-history.csv http://localhost:2013/api/wallet/txs.csv

# OS autostart status (configured=login expects OS registration)
curl -s http://localhost:2013/api/autostart | jq .

# Cross-platform CLI equivalent (no dashboard required)
dogego cert autostart
dogego cert autostart -json
dogego cert autostart -conf /path/to/dogecoinconf.json
dogego cert founder
dogego cert founder -json
dogego cert founder -conf /path/to/dogecoinconf.json -datadir /path/to/chain-data
dogego cert provision
dogego cert provision -json -offline
dogego cert provision -preflight
dogego cert provision -preflight -run-setup -mine-bootstrap
dogego cert preflight
dogego cert preflight -json -require-core
dogego cert preflight -json -require-core -require-wallet-dat
dogego cert weekly
dogego cert weekly -json -require-wallet-dat
dogego cert weekly-live
dogego cert weekly-live -mine-bootstrap -require-wallet-dat -include-long-soak
dogego cert weekly-live -skip-scripts -mine-bootstrap -json
dogego cert live-soak
dogego cert live-soak -duration-min 60 -require-soak-env -json
dogego cert live-soak -skip-scripts -json
dogego cert setup-parity
dogego cert setup-parity -mine-bootstrap -json
dogego cert wallet-migration
dogego cert wallet-migration -wallet-dat "C:\path\wallet.dat" -passphrase "yourpass" -json
dogego cert wallet-import
dogego cert wallet-import -json
dogego cert operator
dogego cert operator -skip-field-evidence -skip-wallet-import -json
dogego cert pq
dogego cert pq -json
dogego cert enable-weekly
dogego cert enable-weekly -weekly-only -dry-run -repo owner/name
```

Offline prerequisite bundle (ROADMAP exit checklist): `scripts/cert_offline_prerequisites.ps1` (`-IncludePQ`, `-IncludeOperator`) or `./scripts/cert_offline_prerequisites.sh` (`INCLUDE_PQ=1`, `INCLUDE_OPERATOR=1`).

dogego-live full sequence: [CORE_SIDE_BY_SIDE_WORKFLOWS.md](CORE_SIDE_BY_SIDE_WORKFLOWS.md) (workflow 10).

## Compatibility notes

- **Protocol:** DogeGo follows **Dogecoin Core mainnet consensus rules** (no protocol forks). Storage, RPC subset, and operator surface may differ; see [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md) and ROADMAP **Dogecoin protocol lock**.
- Storage is **not** Core `blocks/` + `chainstate/` - see [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md).
- Mempool persist is **`dogego_mempool.json`**, not `mempool.dat`.
- Wallet is **`wallet.json`**, not Core `wallet.dat` on disk - but **`dogego_importwalletdat`** migrates from Core `wallet.dat` (native BDB read; encrypted via `options.passphrase`; Core `dumpwallet` fallback when `core_rpc_addr` is set). See [WALLET.md](WALLET.md).
- Some RPCs return extra **`dogego_*`** fields or notes; treat them as diagnostic, not Core-stable.
- **`getpeerinfo`** rows include **`bip152_hb_to`** / **`bip152_hb_from`** when BIP152 high-bandwidth mode is negotiated on that link (Core-shaped; DogeGo caps HB peers at 3). Operator probe: `scripts/core_bip152_probe.ps1` or `GET /api/core-bip152-probe` (included in reboottestnet E2E runbooks and `GET /api/core-end-to-end-probe`).

## Next documentation (ROADMAP Phase 12)

- OpenAPI / Swagger for JSON-RPC
- Language SDK samples (Node, Rust)
- Webhook (not implemented). **ZMQ:** set `zmqpubhashblock`, `zmqpubhashtx`, `zmqpubrawblock`, `zmqpubrawtx` in `dogecoinconf.json` (Core-compatible PUB; restart required).
