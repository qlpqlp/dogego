# DogeGo operator notes

DogeGo is Beta node software. For production wallets, mining pools, and exchange integrations, use [Dogecoin Core](https://github.com/dogecoin/dogecoin).

## Data layout

Per network under your datadir:

| Path | Purpose |
|------|---------|
| `mainnet/` or `testnet/` | Chain-specific data |
| `headers/` + `headers_sync.json` | Header journal (segment layout: `headers/seg/*.bin`, `headers/manifest.json`; legacy `headers.bin` migrates on open) |
| `headers_aux.bin` | AuxPoW sidecar (when used) |
| `rawblocks/` | Downloaded block payloads |
| `banlist.json` | Persistent `setban` entries |
| `misbehavior_scores.json` | Pre-ban misbehavior scores |
| `learned_addrs.json` | Addrbook v2 |
| `block_peer_scores.json` | Block download peer scoring |
| `dogecoinconf.json` | Merged config (includes persisted `addnode`, RPC, sync knobs) |
| `indexes/tx/` | Transaction index (optional; disable with `no_tx_index`) |

DogeGo uses **native storage only** - it does not read Dogecoin Core `blocks/` or `chainstate/`. Migrate by syncing from the network (or copy DogeGo’s own `headers.bin` / `rawblocks/` trees between machines).

## Mainnet

| Topic | Guidance |
|--------|----------|
| Network | `"network": "mainnet"` (or `-network mainnet`). P2P port **22556**, magic `c0 c0 c0 c0`. |
| Discovery | DNS seeds `seed.multidoge.org`, `seed2.multidoge.org` plus fixed seeds from Core `chainparamsseeds.h`. If DNS lookup fails (common on some networks), DogeGo continues with fixed seeds only. CLI **`-dnsseed=0`** or `"dnsseed_lookup": false` skips DNS entirely (Core `-dnsseed`). |
| Wallet | Built-in `wallet.json` under `mainnet/` (BIP44 `m/44'/3'/…`, same as Core). Use `-nowallet` to disable. |
| Sync | Full node needs `rawblocks/` + optional tx index; wallet balances use the in-memory **UTXO cache** (`rescan` calls `SyncUtxo`). |

## SPV mode

- Profile **SPV + wallet** (or `-mode spv` / `FullNode=false`): headers sync without storing `rawblocks/`.
- With wallet enabled, DogeGo builds a **BIP37 bloom filter** from watched scripts, sends `filterload` to `NODE_BLOOM` peers, requests `MSG_FILTERED_BLOCK`, and ingests matched txs into wallet history. Peers without bloom still work for headers; BIP157 compact filters are preferred when advertised.
- Full-node DogeGo advertises `NODE_BLOOM` so libdogecoin / Core SPV clients can sync against it the same way.

## Wallet encryption

- `encryptwallet "passphrase"` - encrypts spend keys in `wallet.json` and locks the wallet.
- After encrypting, restart and run `walletpassphrase "passphrase" 600` before `sendtoaddress` / `dumpprivkey`.
- `walletlock` clears keys from memory; timeout auto-locks in the background.
- Format is **not** Core `wallet.dat` on disk; back up `wallet.json` on encrypted media.
- **Receive tab backup reminder:** the dashboard prompts you to download `wallet.json` when no backup was saved in this browser for **30 days** (`localStorage` key **`dogego_backup_last_download`**). **Dismiss 30 days** snoozes the banner. Use **Download backup** on Receive or `backupwallet` / Console export; encrypt with Settings → Wallet or `encryptwallet`.
- **Migrating from Core:** probe with `dogego_probewalletdat` (includes `pool_count`, `pool_pubkeys`, `pool_entries` with `spends_key_matched`, `pool_keys_matched`/`pool_keys_unmatched`, `pool_unmatched_entries`, `pool_unmatched_hint`, `pool_index_min`/`pool_index_max`, `pool_indices_replayed` for Core keypool entries), import with `dogego_importwalletdat` (native BDB; returns **`keypool_hint`**, **`keypool_refill_size`** when pool-only rows remain, and **`pool_indices_replayed`** when matched HD receive pubkeys replay on import; encrypted wallets accept `options.passphrase`). Configure `core_rpc_addr` in Settings → Advanced for Core `dumpwallet` fallback. See [WALLET.md](WALLET.md), **Core wallet.dat keypool** below, and `dogego cert wallet-migration`.

## Core wallet.dat keypool (migration)

Core Berkeley DB `wallet.dat` files may contain pre-generated **pool** pubkeys (separate from issued spend keys). DogeGo reports pool metadata on probe and import; HD keypool lives in `wallet.json`, not Core’s on-disk pool file.

| Field | Meaning |
|-------|---------|
| `pool_keys_matched` | Pool pubkey also appears as a spend key (`ckey`/`key`) in wallet.dat - imports with the key |
| `pool_keys_unmatched` | **Pool-only** pubkeys with no spend key in wallet.dat - private key material is not in the BDB extract; DogeGo cannot recover these keys |
| `pool_unmatched_entries` / `pool_unmatched_hint` | The pool-only rows and operator guidance shown on probe/import |
| `pool_indices_replayed` | `true` after native import when `wallet/pool_replay.go` reserved matched BIP44 receive pubkeys into `hd_keypool` (always `false` on probe-only) |
| `pool_core_indices_stored` / `hd_keypool_core_index` | Core pool index numbers stored for matched HD receive keys in `wallet.json` |
| `pool_index_min` / `pool_index_max` | Core pool index range from BDB |
| `keypool_refill_size` | Import requested this HD `keypoolrefill` size because pool-only rows remained |
| `keypool_hint` | Operator guidance when pool entries are present |
| `iskeypool` / `hd_keypool_core_index` | Per-address keypool metadata on **`getaddressinfo`**, **`validateaddress`**, and **`dogego_listwalletaddresses`** (Receive address book); **`GET /api/core-wallet-probe`** reports **`address_book_keypool_count`** / **`address_book_core_pool_indices_stored`** |

**Recommended workflow:** probe first (`dogego_probewalletdat`, Receive tab, or `GET /api/core-wallet-probe` when `DOGEGO_WALLET_DAT` is set), then `dogego_importwalletdat`. If `pool_keys_unmatched` > 0, follow `pool_unmatched_hint`; import will top up fresh HD receive keys and may return **`keypool_refill_size`**. Pool-only rows are expected and do not mean import failed. Certification: **`dogego cert wallet-import`** (offline superset) or **`dogego cert wallet-migration`** (`-offline-only` for fixtures only; `-wallet-dat`, optional `-live-probe` / `-live-import` on dogego-live).

## Solo mining and wallet UI performance

When **auto-mining** on reboot testnet (`mine=true`), the built-in wallet can accumulate hundreds of coinbase UTXOs.

**Dashboard (recommended):** the web UI reads wallet balance, transaction history, CSV export, and Send coin control from the **in-memory UTXO cache** (`/api/wallet`, `/api/wallet/txs`, `/api/wallet/utxos`) - the same source BlockStep uses. `GET /api/wallet` and **`GET /api/summary`** expose **`keypool_size`**, Core pool index metadata, **`wallet_index_height` / `needs_rescan`**, **`wallet_scan_index_ok`** when **`wallet.db`** is indexed through the chain tip, and **`wallet_history_fast_path`** when receive rows exist but the index lags tip (listtransactions already fast; incremental rescan still backfills fee/hex). When caught up and **`needs_rescan`** is set, the dashboard auto-starts **one incremental rescan per browser session** (or use **Settings → Wallet**). The same auto-rescan also runs when **`wallet_listtransactions_utxo_walk`** is set and the wallet has **more than 64** spendable UTXOs (typical solo-miner coinbase backlog before the first **`wallet.db`** scan index exists). While **`wallet_listtransactions_scan_pending`** is set, the History tab shows a defer message instead of walking every UTXO; history reloads automatically when the scan index becomes ready. **`wallet_history_deferred`** and **`wallet_history_defer_reason`** on **`GET /api/wallet`** / **`GET /api/summary`** (and **`dogego_wallet_history_defer_reason`** on **`getwalletinfo`**) use the same defer codes as **`GET /api/wallet/txs`**. Coin control matches all wallet **SpendScripts** (HD receive/change indices), not only the default address. Rebuild and restart `dogego` after upgrades, then hard-refresh the browser (**Ctrl+Shift+R**).

**Restart on caught-up solo nodes:** when `chain_active.manifest.json` and **`utxo.cache`** match contiguous bodies, startup skips a full **`SyncUtxo`** connect pass. A short **connect lag** worker may still link a few trailing blocks after restart; the dashboard defers heavy wallet polls while **`dogego_connect_lag`** is high ([WEB_UI.md](WEB_UI.md)).

**External signer (HWI):** set **`signer_cmd`** in `dogecoinconf.json` (stdin/stdout JSON, HWI-compatible). Use `enumeratesigners` and Features → **Wallet probe** (`signer_cmd_configured`, device count). Funded PSBTs include BIP32 deriv paths (`psbt_bip32_deriv_ok` on probe). Native USB/HID without HWI is not supported. See [WALLET.md](WALLET.md).

**JSON-RPC (solo miner):** `getwalletinfo`, `getbalance`, and **`listunspent`** read the same UTXO cache (with persisted **`wallet_utxo_scan.cache.json`** across restarts). **`listtransactions`**, **`gettransaction`**, and **`listsinceblock`** use the UTXO-cache light path with a short in-process row cache; when **`wallet.db`** scan history includes receive rows, **`listtransactions`** skips the per-UTXO receive walk (solo-miner history stays under the 3s probe gate). Confirmed wallet send **`fee`** and **`hex`** are stored in **`wallet.db`** at broadcast and block scan (no block load on compact tx index). History indexed before this build may need **`rescan`** to backfill fee/hex.

**Console / external RPC:** enable `"tx_index_embed_tx": true` in `dogecoinconf.json` when you need **`gettransaction` hex** for txs that were never wallet-indexed (imported watch-only activity, pre-rescan history).

**Solo mining confirms sends:** the background miner (`mine=true`, 15s interval) includes **mempool transactions** in each block (same selection as `getblocktemplate`), so wallet sends confirm without manual `generatetoaddress`.

**Spending mined coins:** coinbase outputs need **~240 confirmations** on modern testnet before they are spendable (`sendtoaddress` uses mature UTXOs only; immature balance shows in the Send card).

## Web dashboard & setup wizard

- Default dashboard: **`http://localhost:2013/`** (`webui` in config) - use **`localhost`** (not `127.0.0.1`) when you want optional **device biometrics / WebAuthn**. `"webui": "127.0.0.1:2013"` still works without biometrics; local HTTPS certs include both names.
- First run without `datadir`: **setup wizard** on the same port - last step has **Save** and **Save & start node** (green); starting the node opens the dashboard **in the same browser tab**, not a new window.
- **Login autostart:** set `"autostart": "login"` in `dogecoinconf.json` (or enable on the wizard finish step / **Settings → Interface**). DogeGo registers OS sign-in startup (Windows Task Scheduler, Linux `systemctl --user` or XDG `.desktop`, macOS LaunchAgent). Runs `dogego node -nobrowser` with your saved config - chain data stays in `datadir`. Headless Linux may need `loginctl enable-linger $USER`. Status: `GET /api/autostart`, Settings hint line, or **`dogego cert autostart`** (add `-json` for machine-readable output; `-conf PATH` to check a specific config file).
- Tab reference (Overview, Send, Explorer, Mempool, Settings, …): **[WEB_UI.md](WEB_UI.md)**.

## RPC and web UI security

- Bind RPC to **loopback** (`127.0.0.1`) and the web dashboard to **`localhost`** (default) or `127.0.0.1` unless you deliberately expose them.
- Enable `rpc_user` / `rpc_password` or `rpc_cookie` in `dogecoinconf.json`.
- Add `rpcallowip` entries (JSON array) only if RPC must accept non-loopback clients, e.g. `"rpcallowip": ["192.168.1.0/24"]`. Loopback is always allowed.
- Optional `rpcwhitelist` limits callable methods when set, e.g. `"rpcwhitelist": ["getblockchaininfo", "getpeerinfo", "getmempoolinfo"]` (`ping` / `help` / `uptime` / `getrpcinfo` always allowed).
- Optional `"rpclimit": 600` caps JSON-RPC POSTs per client IP per minute; `"rpcauthmaxfail": 30` limits failed Basic auth attempts (default **30** when auth is enabled; use **-1** to disable the auth-fail cap only).
- Do not expose unauthenticated RPC to the internet.

### TLS

**Optional native TLS** (TLS 1.2+) in `dogecoinconf.json` - set **both** paths for each listener you want encrypted:

| Keys | Listener |
|------|----------|
| `rpc_tls_cert`, `rpc_tls_key` | JSON-RPC (`rpc` address) |
| `webui_tls_cert`, `webui_tls_key` | Web dashboard (`webui` address) |

Use PEM files (e.g. from Let’s Encrypt). The dashboard opens `https://…` when `webui_tls_*` is set.

**Auto-generated local HTTPS** (development / loopback): set `webui_tls_local` and/or `rpc_tls_local` to generate a local CA and leaf certs under `{datadir}/tls/`. HTTP remains the default when these are off. To avoid browser warnings, set `local_tls_trust_ca=true` on startup or run `dogego tls trust-ca` (also available from Settings → Interface → Local HTTPS). See [SECURITY.md](SECURITY.md#local-https-optional).

**Disable TLS entirely** (DogeBox / first install without cert support): `dogego node -notls` or `DOGEGO_NO_TLS=1`. Skips wizard HTTPS, cert generation, and OS CA install.

**Recommended for production:** still run `dogego` on localhost and put **nginx**, **Caddy**, or **Traefik** in front with TLS, auth, and IP restrictions.

Example nginx location (illustrative):

```nginx
location / {
    proxy_pass http://127.0.0.1:22555/;
    proxy_set_header Authorization $http_authorization;
}
```

## Reboot testnet (first node / small network)

DogeGo’s default **`network": "testnet"`** is the **rebooted** chain from parent `CTestNetParams` (new genesis, P2P magic `fd d4 dc e1`, port **44556** - same port number as legacy testnet3, **different chain**).

| Topic | Guidance |
|--------|----------|
| First node | `mine=true`, wallet or `miningaddress`, `p2p_connectivity=both`. Node enters **solo founder** mode if seeds time out, mines locally, and still runs **listen + outbound** when multi-peer mode is enabled. |
| Second node | `"addnode": ["FOUNDER_HOST:44556"]` in `dogecoinconf.json` (or `peer=` for one-shot). Founder should forward **44556** if on the public internet. |
| Discovery | Built-in DNS seed **`seed.dogego.org` first** (DogeBox running a DogeGo reboot-testnet full node, for quick peer discovery), then optional extra **`dnsseed`** hostnames in `dogecoinconf.json`, then Core fixed seeds from `chainparamsseeds.h`. Growth is also **addnode**, inbound connect, and **`getaddr`**. |
| Datadir | Use a **fresh** `testnet/` folder; do not reuse old testnet3 `blocks/` / `chainstate/`. After **consensus param** changes (Digishield/min-diff/subsidy from block 1), wipe `testnet/` or peers must run the same build. |

### Founder checklist (solo first node)

Use this when you are the **first** operator on reboot testnet (no peers yet) or when standing up a small private test network.

1. **Fresh datadir** - empty `testnet/` under your `datadir` (or let the setup wizard create one). Never point at legacy testnet3 chain folders.
2. **Config** - `network=testnet`, `mine=true`, wallet enabled (wizard default), `p2p_connectivity=both` so the node listens on **44556** and still makes outbound attempts. The setup wizard **Finish** step runs the same founder checks as `dogego cert founder` (`POST /api/setup/founder-preflight`).
3. **Start** - `dogego node` (wizard **Save & start node**) or `dogego node -datadir DIR -nobrowser`. Logs should show **solo founder** / local mining when seeds time out.
4. **Verify** - `getblockchaininfo`: `blocks` increases; dashboard **Overview** shows tip advancing. Mining rewards credit the built-in wallet (History → Mining). Features → **Founder probe** or `GET /api/core-founder-probe` when the node is running.
5. **Share peer** - give joiners your reachable host as `HOST:44556` (forward TCP **44556** on routers/firewalls for public founders).
6. **Joiners** - they usually need no manual peer: DNS hits **`seed.dogego.org` first**, then Core fixed seeds. For a private founder, still use `"addnode": ["HOST:44556"]` (or one-shot `-peer HOST:44556`). Growth is also inbound connects and **`getaddr`**.
7. **Optional extra DNS seed** - publish A/AAAA for your own hostname and set `"dnsseed": ["seed.example.com"]` (appended after `seed.dogego.org`) so installs can discover your founder without addnode.
8. **Optional autostart** - `"autostart": "login"` (wizard / Settings) so the node resumes after OS sign-in; verify with **`dogego cert autostart`** or Features → **OS login autostart** cert gate.
9. **After consensus upgrades** - wipe `testnet/` on every node or ensure all peers run the **same DogeGo build** (Digishield, min-diff, subsidy params must match).

**Example founder `dogecoinconf.json` (minimal):**

```json
{
  "network": "testnet",
  "datadir": "C:\\Users\\you\\dogego-data",
  "mine": true,
  "p2p_connectivity": "both",
  "rpc_cookie": true,
  "webui": "localhost:2013"
}
```

**Example joiner snippet:**

```json
{
  "network": "testnet",
  "datadir": "C:\\Users\\you\\dogego-joiner",
  "addnode": ["founder.example.com:44556"],
  "p2p_connectivity": "both"
}
```

**Certification** - offline gates: `dogego cert offline` (`scripts/ci_offline_gate.{ps1,sh}`). Milestone E standalone operator cert: **`dogego cert operator`** (consensus/store/node/rpc + field-evidence + wallet-import; `scripts/operator_workflow_cert.{ps1,sh}`). Wallet import (BIP39/BIP38 + signer + wallet.dat): **`dogego cert wallet-import`** (`scripts/wallet_import_cert.{ps1,sh}`). PQ format/carrier (optional): **`dogego cert pq`** (no production PQ safety claim; `scripts/pq_cert.{ps1,sh}`). Milestone A field evidence: **`dogego cert field-evidence`** (`scripts/field_evidence_cert.{ps1,sh}`; regen/auxpow export flags on PS1). Founder preflight: **`dogego cert founder`** (optional `-conf` / `-datadir`). Core **wallet.dat** live probe/import: **`dogego cert wallet-migration`** (optional `-offline-only` for fixtures; `-wallet-dat` / `-passphrase`; `-live-probe` / `-live-import` for RPC on a running node; `scripts/wallet_migration_cert.{ps1,sh}`; env `DOGEGO_WALLET_DAT` / `DOGEGO_WALLET_DAT_PASSPHRASE`). **dogego-live runner:** **`dogego cert provision`**, **`dogego cert preflight`**, **`dogego cert weekly`**, then **`dogego cert enable-weekly -require-wallet-dat`** (sets `DOGEGO_SCHEDULED_WEEKLY_LIVE` and optional `DOGEGO_WALLET_DAT_REQUIRED` via `gh`). One-shot bootstrap: **`dogego cert provision -preflight -run-setup -mine-bootstrap`** (or `ci_runner_provision_checklist.ps1 -RunPreflight -RunSetup`). Reboot testnet Core parity bootstrap (Milestone D): **`dogego cert setup-parity`** (optional `-mine-bootstrap`; mirrors `scripts/setup_reboottestnet_core_parity.ps1`). Prepared runners can run `dogego cert weekly -require-wallet-dat` or dispatch `dogego.yml` with `require_wallet_dat=true`. Full scheduled weekly bundle: **`dogego cert weekly-live`** (mirrors `ci_scheduled_weekly_live.ps1`; optional `-include-long-soak`; **`-skip-scripts`** for preflight-only smoke). Milestone B multi-hour soak: **`dogego cert live-soak`** (mirrors `ci_milestone_b_full_gate.ps1`). Reboot testnet E2E (optional): `scripts/core_e2e_reboottestnet_runbook.ps1` (includes **BIP152 HB** via `core_bip152_probe.ps1`) when Core is side-by-side. Full dogego-live sequence: [CORE_SIDE_BY_SIDE_WORKFLOWS.md](CORE_SIDE_BY_SIDE_WORKFLOWS.md) workflow 10.

## P2P connectivity

- **`p2p_connectivity`**: `both` (default), `classic` (inbound listen + outbound), or `cgnat` (outbound-only).
- **`maxoutbound`**: default **12** (includes primary sync peer); raise for faster block download (cap 32). **`block_sync_workers`**: parallel block-assist TCP sessions (0 = derive from `maxoutbound`, max 24).
- Use **`addnode`** for known-good peers when header sync stalls on dead scored peers.

## Pair two PCs on the same home LAN

DNS seeds and public IP port-forwarding **do not** reliably discover two DogeGo nodes on the **same router** (both PCs share one public IP). Use each machine's **LAN IPv4** and the P2P port:

| Network | P2P port |
|---------|----------|
| Reboot testnet (`network=testnet`) | **44556** |
| Mainnet | **22556** |

**On each PC:**

1. Open **Settings → P2P → Pair with another PC on your LAN** (or `GET /api/lan-peer-hint` / `scripts/lan_peer_pair.ps1 -ShowHint`).
2. Copy **Share this target** (e.g. `192.168.1.10:44556`) and give it to the other operator.
3. On the other PC, enter that PC's LAN IP in **Other PC LAN IP** and click **Add peer now** (runs `addnode HOST:PORT add`), or use Console:

```json
{"method":"addnode","params":["192.168.1.10:44556","add"]}
```

4. **Mutual pairing** - repeat on the first PC with the second PC's LAN target so both nodes dial each other.
5. Verify with `getconnectioncount` / `getpeerinfo` or Overview → Network.

**PowerShell helper:** `scripts/lan_peer_pair.ps1 -OtherHost 192.168.1.214` (adds default port from config). `-Mutual` prints the reverse command for the other machine.

**Firewall:** allow inbound TCP on the P2P port on each PC (Windows Defender Firewall → allow `dogego.exe` or the port).

**Remote dashboard (optional):** to view Analytics from another PC on your LAN, set `"webui": "0.0.0.0:2013"`, enable **`webui_remote_auth`** and a dashboard PIN in Settings → Interface, then open `http://LAN_IP:2013` and unlock.

**Do not** use the router's public IP as `addnode` when both nodes are on the LAN - the forward rule typically reaches only one host.
- Optional **`dnsseed`**: JSON array of hostnames (A/AAAA → default network port), e.g. `"dnsseed": ["seed.mytestnet.example"]` for a founder-run DNS seed on reboot testnet.
- **`maxorphantx`**: orphan pool cap (default 100, max 1000).
- **BIP152 compact blocks (v1):** `sendcmpct` negotiates high-bandwidth mode with up to **3** persistent peers; `getpeerinfo` exposes **`bip152_hb_to`** / **`bip152_hb_from`**. Pre-auxpow blocks use `cmpctblock`; AuxPoW blocks fall back to full `inv`/`block`. Probe: `.\scripts\core_bip152_probe.ps1` or Features → BIP152 card (`GET /api/core-bip152-probe`). Offline AuxPoW/cmpct edges: **`dogego cert bip152-soak`** (default `-skip-live`). Optional live timed soak: `dogego cert bip152-soak -skip-live=false` (Windows) or `.\scripts\bip152_live_soak_gate.ps1` / `$env:DOGEGO_BIP152_LIVE_SOAK = "1"` in `core_operator_workflow_cert.ps1` (optional `-RequireRelayActivity` / `DOGEGO_BIP152_SOAK_REQUIRE_RELAY=1`). Go probe is soft on peers-without-HB (notes only); PS1 soak may require HB when not IBD.
- **Protocol lock (solo):** DogeGo does not require Dogecoin Core. Verify buried/BIP9 deployment active-state and getblockchaininfo softforks at tip with `.\scripts\core_protocol_lock_probe.ps1` or `GET /api/core-compare` (`deployment.protocol_lock`). Runs automatically in `core_end_to_end_workflow.ps1` and when `$env:DOGEGO_IBD_SOAK = "1"` in `core_operator_workflow_cert.ps1`. Side-by-side vs Core uses the same checks when `core_rpc_addr` is set.
- **`maxmempool`**: mempool byte cap in MB (default 300; Core `-maxmempool`).
- **`dbcache`**: UTXO / chainstate working-set budget in MB (Core `-dbcache`; **0 = auto** from free RAM). See [Sync speed & validation](#sync-speed--validation-mainnet).
- **`ibd_optimize`**: when not `false`, prioritizes headers/bodies during IBD (default on). See [Sync speed & validation](#sync-speed--validation-mainnet).
- **`mempoolexpiry`**: max transaction age in hours (default 24; Core `-mempoolexpiry`).
- **`persistmempool`**: when `false`, skip auto load/save of `dogego_mempool.json` (default `true`; `savemempool` / `loadmempool` RPC still work).
- **`alertnotify`**: shell command when chain warnings change (Core `-alertnotify`). `%s` is replaced with the warning text, e.g. `"alertnotify": "powershell -Command \"Add-Content -Path alerts.log -Value %s\""` on Windows or `"alertnotify": "echo %s >> /var/log/dogego-alert.log"` on Unix. Notifications are suppressed during forward block IBD (same idea as Core skipping alerts until initial sync).

## Live soak residue (operator-owned)

Offline Core-parity slices for BIP152, P2P eclipse-pressure, mempool DAG packages, BIP125 PaysForRBF descendant conflict fees, **submitpackage CPFP**, utxo.cache crash quarantine, keypool fill-to-target, operator-cert e2e (incl. PQ), and HWI mocksigner are **code-complete**. Remaining green gates need `dogego-live` or a long local soak:

| Gate | Command / script |
|------|------------------|
| BIP152 live HB | `dogego cert bip152-soak -skip-live=false` or `scripts/bip152_live_soak_gate.ps1` |
| Milestone D weekly-live 24/24 | `dogego cert weekly-live` (Core side-by-side on reboottestnet) |
| Milestone B multi-hour soak | `dogego cert live-soak` / `DOGEGO_SCHEDULED_LIVE_SOAK=1` |
| Workflow 10 | `dogego cert workflow10` (enable-weekly → provision → weekly-live → optional live-soak) |
| Disruptive mainnet reindex | Manual sign-off via operator reindex/prune scripts |

See [CORE_SIDE_BY_SIDE_WORKFLOWS.md](CORE_SIDE_BY_SIDE_WORKFLOWS.md) workflow 10 and [CORE_PARITY_GAPS.md](CORE_PARITY_GAPS.md).

## Sync speed & validation (mainnet)

DogeGo uses **headers-first IBD** like Core: headers in `headers.bin`, full blocks in `rawblocks/`, **chainActive** = contiguous bodies from genesis.

| Setting | Effect |
|---------|--------|
| **`assumevalid`** | Core `-assumevalid` analogue. Empty = mainnet default block at height **5,050,000** (skip script checks on older buried blocks). `"0"` = verify every script (slowest, safest). Custom hash = trust that block and ancestors on your header chain. |
| **`checkpoints`** | Core `-checkpoints` analogue (default **on**). Verifies block hashes at Core `mapCheckpoints` heights during header sync (genesis, 104679, 145000, …). Set `"checkpoints": false` only for debugging. |
| **`ibd_optimize`** | Default **on**. Core-style IBD focus: more assist peers, assumevalid script skip when resolved, fewer UTXO flushes, defers analytics sidecar until bodies catch headers, and softens stall handling so download is not peer-churned. Set `"ibd_optimize": false` only for debugging. Restart required. |
| **`dbcache`** | Core `-dbcache` analogue (MB). UTXO working-set budget before flushing `utxo.cache`. **`0` / omitted = auto** (~80% of free RAM after a 2 GB OS reserve, min 256 MB, cap 16 GB). Larger values speed connect catch-up and reduce disk flushes. Restart required. |
| **`maxoutbound` / `block_sync_workers`** | More peers and parallel `getdata` lanes → faster **download**. |

**Download rate on the dashboard:** `blocks_per_minute` is a **recent ~10 minute** window (not the lifetime mean since IBD started). Lifetime average is exposed as `dogego_blocks_per_minute_lifetime` / `blocks_per_minute_lifetime` on raw sync snapshots. Effective auto `dbcache` appears as `dogego_dbcache_mb` once the node has started.

**Header sync recovery:** After a force-kill, DogeGo repairs torn segment tails and drops stale `headers/seg/*.bin.tmp` on startup (legacy `headers.bin` monolith is repaired the same way when present). If peer header timestamps jump far ahead of your local tip (partial mainnet sync), DogeGo **auto-rewinds** to the last 240-block boundary and retries `getheaders`. If you see `bad nBits` at a retarget height, let that rewind run or delete `mainnet/headers.bin` and resync. Reboot **testnet** is the path DogeGo exercises most.

**Dashboard:** Overview + **Sync** tab show body-sync %, blocks behind, blk/min, estimated time left, mempool count, and a one-line status (same data as `GET /api/summary`).

**RPC:** `getblockchaininfo` includes `dogego_assumevalid`, `dogego_assumevalid_height`, `dogego_sync_eta`, `dogego_sync_status`, and body-sync fields. `verificationprogress` is **block-body coverage vs header tip**; `dogego_tx_verification_progress` mirrors Core’s tx-count estimate on mainnet when indexed. `initialblockdownload` also respects Core **`-maxtipage`** (default 86400 s; config `"maxtipage"` or CLI `-maxtipage`). Header diagnostics: `dogego_header_tip_time`, `dogego_header_tip_age_sec`, `dogego_headers_ahead_of_chainactive`, `dogego_header_sync_recovery` (stale `headers.bin` / `bad nBits` hints). Operator recovery: **`dogego_recoverheaders`** (no args) or web **`POST /api/chain/recover-headers`** (Overview button when the recovery hint is shown). Destructive rewind of all chain data to a height: **`truncatetoheight <height>`**. The web **`/api/summary`** exposes the same header diagnostic fields; the Overview banner shows `dogego_header_sync_recovery` when set. The **Console** tab can run JSON-RPC in-process via **`POST /api/rpc`** (loopback only; presets for chain info, mining, header recovery).

**Header sync recovery (mainnet `bad nBits` ~4080 / 10080):** On startup, DogeGo may rewind `headers.bin` when the current difficulty period has unrealistically tight block times (partial sync). During P2P sync it also rewinds on large peer `nTime` jumps or `bad nBits` at retarget, then retries `getheaders`. If sync still stalls, delete `headers.bin` (and optionally `headers_aux.bin` / `rawblocks/`) or use `invalidateblock` when RPC is up.

Run **`verifychain`** (level 4) to force full script verification over a height range regardless of `-assumevalid`.

**Tx index maintenance (native `indexes/tx/`):**

| RPC | Purpose |
|-----|---------|
| **`reindextx`** `[clear]` | Rebuild all tx index files from `rawblocks/` (v2 entries with embedded tx bytes). Optional `true` clears the index first. |
| **`upgradetxindex`** `[max_files]` | Upgrade legacy 36-byte index files to v2 in batches (default 10 000 per call). The node also upgrades 256 files every 15 minutes while running. |

CLI equivalent: `dogego indexer reindex-tx [-clear]`.

## Certification and upgrade scripts

Prefer **`dogego cert …`** on every OS ([scripts/README.md](../scripts/README.md)). Main offline gates have `.sh` and `.ps1` twins. Many IBD helpers are PowerShell-first; on Linux/macOS use the dashboard, `curl`, and `dogego cert ibd-convergence`.

Offline regression gates (no running node):

```bash
cd DogeGo
go run ./cmd/dogego cert operator
# Linux/macOS: ./scripts/operator_workflow_cert.sh
# Windows:     .\scripts\operator_workflow_cert.ps1
```

Live mainnet health (node running):

```bash
curl -sS "http://localhost:2013/api/core-operator-cert?refresh=1"
# Windows helper: DOGEGO_IBD_SOAK=1 .\scripts\ibd_soak_cert.ps1
```

**Post-aux header stall (~510k / ~8% UI):** after upgrading from builds that rejected valid aux parent chain IDs, restart with a current `dogego` binary - no `headers/` wipe required. See [CORE_OPERATOR_RUNBOOK.md](CORE_OPERATOR_RUNBOOK.md) and [CORE_SIDE_BY_SIDE_WORKFLOWS.md](CORE_SIDE_BY_SIDE_WORKFLOWS.md) workflow 4b.

| Tool | Purpose |
|------|---------|
| `dogego cert …` | Cross-platform certs (offline, operator, mining, ibd-convergence, …) |
| `dogego_rpc.ps1` | Windows HTTP JSON-RPC helper (use **curl** on Linux/macOS) |
| `upgrade_post_aux_verify.ps1` | Windows: auxpow parent chain ID parity (or Features mining probe) |
| `check_header_progress.ps1` | Windows: exit 0 when `headers` ≥ 510000 (or Console `getblockchaininfo`) |
| `watch_sync.ps1` | Windows: poll RPC during IBD (or Overview sync dock) |
| `log_ibd_progress.ps1` | Windows: CSV soak log (`-DiskOnly` when RPC off) |
| `sync_status.ps1` | One-shot disk checkpoints + optional RPC (`-Json` for scripts) |
| `node_health.ps1` | One-shot health gate (RPC, auxpow parity, connect lag, process lock) |
| `restart_node.ps1` | Windows: stop all `dogego`, optional `-Rebuild`, start one instance |
| `check_update.ps1` / `check_update.sh` | Query GitHub for a newer release |
| `schedule_update_check.ps1` / `.sh` | Daily update check (Task Scheduler or cron) |
| `ibd_convergence_check.ps1` | Windows timed IBD window (`dogego cert ibd-convergence` on any OS) |
| `GET /api/core-ibd-convergence-probe` | Single IBD progress snapshot (headers, contiguous bodies, connect boost); timed window via `ibd_convergence_check.ps1` |
| `GET /api/core-addrman-probe` | Partial Core addrman snapshot (`getaddrmaninfo` + chaininfo cross-check); **16th** live operator-cert gate; mirrors `scripts/core_addrman_workflow.ps1` |
| `GET /api/wallet/txs` | Paginated history; returns **`deferred`** + **`defer_reason`** during IBD, connect lag, or scan index build (>64 UTXOs); same codes on **`GET /api/wallet`**, **`GET /api/summary`**, and **`getwalletinfo`** (`wallet_history_defer_reason` / **`dogego_wallet_history_defer_reason`**) |
| `dogego cert ibd-convergence` | Cross-platform IBD progress check (mirrors `ibd_convergence_check.ps1`; `-json`, `-interval-sec`, `-disk-only`) |
| `ibd_monitor.ps1` | Health + sync status; optional `-ConvergeSec 120` progress proof |
| `ibd_snapshot.ps1` | One CSV row for Task Scheduler (pairs with `ibd_progress.csv`) |
| `ibd_progress_report.ps1` | Summarize deltas from `ibd_progress.csv` |
| `resume_node.ps1` | Start `dogego` when RPC is down (refuses if already running) |

Operator scripts call **JSON-RPC over HTTP** (same as `curl` / `dogecoin-cli`), not `dogego` subcommands. The node must be running with RPC enabled (default `http://127.0.0.1:22555/`).

`getblockchaininfo` fields: `dogego_auxpow_parent_chain_id_core_parity`, `dogego_post_aux_era_header_stall`, `dogego_header_sync_recovery`, `dogego_stored_bodies_ahead_connect`, `dogego_connect_lag`, `dogego_connect_blocks_per_minute`, `dogego_raw_blocks_ahead_of_contiguous`, `dogego_raw_sync.blocks_per_minute`, `warnings` (Core-shaped).

## Auto-update

DogeGo checks [GitHub Releases](https://github.com/qlpqlp/dogego/releases) for a newer version (default: every **24 hours**; manual check from the system tray or `dogego version` when online).

| Control | Where |
|---------|--------|
| Overview banner | **View release**, **Download binary**, **Install update**, **Dismiss** |
| System tray | Check / download / install / dismiss (desktop builds) |
| Disable checks | `DOGEGO_NO_UPDATE_CHECK=1` (or `true` / `yes`) |

**Download:** `POST /api/update/download` (loopback) saves a platform-matching asset under `<datadir>/updates/` (for example `dogego-0.2.0.exe`). When the release publishes a **`.sha256`** sidecar, DogeGo verifies the file before keeping it.

**Install update:** `POST /api/update/apply` downloads if needed, verifies SHA256 when published, spawns the new binary with **`-waitpid=<current>`**, stops this process, restarts, and copies the new binary over the install path (**`-replacetarget`**). Use **Settings → Services → Restart node** for config-only restarts without upgrading.

**Publishing releases:** push a tag `v*` (for example `v0.2.0`). GitHub Actions workflow **`.github/workflows/release.yml`** builds `dogego-windows-amd64.exe`, `dogego-linux-amd64`, `dogego-darwin-amd64`, `dogego-darwin-arm64`, and matching `.sha256` files.

After any upgrade, hard-refresh the browser (**Ctrl+Shift+R**) so the dashboard loads new static assets.

**CLI:** `dogego version` prints build info and may note an available release. `dogego version -check` queries GitHub now (exit **2** when an update exists). `dogego version -json` prints machine-readable version + update fields. **`scripts/check_update.ps1`** / **`check_update.sh`** wrap the same check for Task Scheduler or cron. **`scripts/schedule_update_check.ps1 -Register`** (Windows) or **`scripts/schedule_update_check.sh --install-cron`** (prints a cron line) schedule a daily check when the node is not running 24/7.

On desktop builds, a **native OS notification** (Windows balloon, Linux `notify-send`, macOS Notification Center) appears once per release when an update is newly detected (in addition to the Overview banner and tray tooltip).

## dogego-live workflow 10 (scheduled CI)

One-shot orchestration for the self-hosted **`dogego-live`** runner (mirrors [CORE_SIDE_BY_SIDE_WORKFLOWS.md](CORE_SIDE_BY_SIDE_WORKFLOWS.md) workflow 10):

```powershell
# Preflight only (no scripts)
dogego cert workflow10 -skip-scripts

# Enable GitHub repo vars for weekly + optional soak (requires gh CLI)
dogego cert enable-weekly -repo owner/dogego

# Full sequence on the runner (provision → setup-parity → weekly-live → optional live-soak)
dogego cert workflow10 -mine-bootstrap
```

Web UI: **Features → CI runner readiness** (`GET /api/core-runner-probes`) and **Workflow 10 preflight** (`GET /api/core-workflow10-probe`). Set repo variables `DOGEGO_SCHEDULED_WEEKLY_LIVE=1` and optionally `DOGEGO_SCHEDULED_LIVE_SOAK=1` after `dogego cert enable-weekly`.

## JSON-RPC

- Single requests and **batch arrays** are supported on `POST /`.
- `stop` triggers graceful shutdown when wired from `dogego node`.
- WebUI / systray / `stop` cancel the node immediately, flush on a short budget (~20s), then force-exit if still stuck. An unclean exit leaves `.dogego-unclean-shutdown` under the chain datadir; the next start runs a synchronous repair pass over headers/UTXO/indexes.
