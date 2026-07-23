# DogeGo web dashboard & setup wizard

The built-in HTTP UI is **Beta** and intended for **loopback** use (`localhost` by default). It is not a replacement for Dogecoin Core's Qt wallet UI.

**Protocol lock:** the dashboard reflects DogeGo's Core-aligned mainnet consensus policy (no protocol forks). Features tab **Core guidance** and parity gaps include the `protocol_lock` row; offline cert: `dogego cert offline` or `scripts/cert_offline_prerequisites.{ps1,sh}`.

## URLs

| Surface | Default listen | Purpose |
|---------|----------------|---------|
| **Dashboard** | `localhost:2013` | Main operator UI (`index.html`) |
| **Setup wizard** | Same port during first run | Shown when `datadir` is missing (`setup.html`) |
| **Analytics (legacy page)** | `/analytics.html` | Redirects to dashboard Analytics tab |

Configure with `webui` in `dogecoinconf.json` or `-webui`. Disable with `nowebui` / `-nowebui`.

**Default `localhost` vs `127.0.0.1`:** the default is **`localhost:2013`** so optional **device biometrics (WebAuthn)** work in browsers on Windows, Linux, and macOS. You may set `"webui": "127.0.0.1:2013"` if you do not need biometrics. Local HTTPS (`webui_tls_local`) certificates include **both** `localhost` and `127.0.0.1` (and `::1`) as SANs, so either URL works over HTTPS. When `webui` is `localhost:…`, DogeGo binds IPv4 and IPv6 loopback so both names reach the dashboard.

Open the dashboard at **`https://localhost:2013/`** when local HTTPS is on, or **`http://localhost:2013/`** when TLS is off (including **`-notls`** / DogeBox).

## Local HTTPS and `-notls`

New wizard installs default to **HTTPS** (`webui_tls_local` + `local_tls_trust_ca`) and may install a local CA into the OS trust store. Toggle these under **Settings → Interface → Local HTTPS**.

| Mode | How | Effect |
|------|-----|--------|
| Local HTTPS (default wizard) | `webui_tls_local` / Settings | Certs under `{datadir}/tls/`; open `https://…` |
| Trust CA | `local_tls_trust_ca` or `dogego tls trust-ca` | Fewer browser warnings |
| **Plain HTTP** | **`dogego node -notls`** or **`DOGEGO_NO_TLS=1`** | No cert gen, no CA install; wizard + dashboard stay `http://` |

Use **`-notls`** on hosts that cannot terminate TLS (e.g. **DogeBox** pup entrypoint already passes it). The flag also overrides TLS flags already saved in `dogecoinconf.json` for that process. Persisted `"no_tls": true` from a `-notls` wizard save keeps HTTP on later starts.

See [SECURITY.md](SECURITY.md#local-https-optional) and [OPERATOR.md](OPERATOR.md#tls).

## First-run setup wizard

1. Run `dogego node` without `-datadir` (or without `datadir` in config).
2. On a **desktop** session (Windows/macOS, or Linux with `DISPLAY` / `WAYLAND_DISPLAY`), DogeGo **opens the setup wizard in your default browser** automatically. Headless servers print the URL instead; set `DOGEGO_HEADLESS=1` to force headless behavior on a desktop.
3. Five steps: **Profile** → **Data** → **Network** → **Sync** (skipped in SPV) → **Finish**.
4. On the last step:
   - **Back** - previous step only (no **Next**).
   - **Save & start node** (green) - saves config, starts the node process, and navigates to the **dashboard in the same tab** (not a new browser window).
5. **Do not open browser automatically** is unchecked by default on desktop so manual `dogego node` starts open the dashboard; the setup wizard still navigates in the same tab after **Save & start node** (no extra window). **System tray** is checked by default on desktop installs.
6. **P2P identity (Network step):** optional **user-agent comment** plus an optional **tip address** published in the wire user-agent (visible to every peer). You can generate a **dedicated DogeGo HD tip key** at `m/44'/3'/0'/2/0` (separate from Receive) or paste your own address. A **wire user-agent preview** updates as you type (setup wizard and Settings).
7. **Reboot testnet founders:** on **Finish** (testnet + full node), the wizard runs **founder preflight** (`POST /api/setup/founder-preflight`) - same checks as `dogego cert founder`.
8. **Start at sign-in:** when enabled, `POST /api/setup` always saves config first. If OS task registration fails (e.g. Windows **Access is denied** without Administrator), the response is still `ok: true` with **`autostart_warning`** - uncheck sign-in autostart or run once as Administrator; start the node manually if needed.
9. **Reboot testnet peers:** default discovery uses DNS seed **`seed.dogego.org` first** (a DogeBox running a public DogeGo reboot-testnet full node so new nodes find peers quickly), then Core fixed seeds, then any extra `"dnsseed"` you set in Settings. See [CHAIN_PARAMETERS.md](CHAIN_PARAMETERS.md) and [OPERATOR.md](OPERATOR.md).

Language picker (top bar): English, French, Portuguese (Portugal), German, Chinese, Japanese - locale files under `ui/static/locales/`.

**Boot overlay:** on dashboard load, a short "Much prepare. Very load." screen hides once chain summary data is available (`dogego_sync_ok` or active IBD with a known tip). If the wallet is enabled, the overlay may wait briefly for a receive address unless sync is heavy (IBD). Hard refresh never blocks longer than ~45s (partial data) or ~90s (absolute cap); wallet transaction history continues loading in the background.

**Custom URL (`dogecoin://`):** on desktop sessions DogeGo **re-registers** the per-user `dogecoin://` handler on each start (Windows HKCU - replaces Dogecoin Core's handler for your user account). You can also run `dogego register-url-protocol` manually. Supported links:

| URI | Action |
|-----|--------|
| `dogecoin://node` | Open the local dashboard |
| `dogecoin://node/send` | Dashboard Send tab (`#send`) |
| `dogecoin:ADDRESS` or `dogecoin://ADDRESS` | Send tab with address prefilled (BIP21 `amount`, `label`, `message` query params supported) |

The node must be running for payment links; `dogego open --url %1` is invoked by the OS handler.

**System tray:** enabled by default on desktop when `tray` is omitted from config. Override with `"tray": false`, `-tray=false`, or the setup wizard / Settings checkbox. Tray menu: running **version** line, **Open Dashboard**, **Open Console**, **View activity logs**, **Check for updates**, **Download update**, **Install update** (when a release asset exists), **View release on GitHub**, **Dismiss update notice**, **Shutdown Node**. On Windows, minimizing or closing the console window hides it to the tray instead of stopping the node (restore from the taskbar to show the console again).

**Auto-update:** when a newer release is on [GitHub Releases](https://github.com/qlpqlp/dogego/releases), Overview shows a banner with **View release**, **Download binary**, and **Install update**. Downloads land under `<datadir>/updates/` with optional **SHA256** verification from a `.sha256` sidecar. **Install update** spawns the new binary, waits for this process to exit, restarts, and replaces the install path. Desktop builds also show a **native OS notification** once per newly detected release (Windows balloon, Linux `notify-send`, macOS Notification Center). Disable checks with `DOGEGO_NO_UPDATE_CHECK=1`. See [OPERATOR.md](OPERATOR.md) § Auto-update.

## Dashboard tabs

### Overview
- **Update available** banner when GitHub Releases has a newer version (dismissible; rechecks daily).
- Blockchain sync: header tip vs **contiguous block bodies** (chainActive); hero shows **Downloading headers**, **Downloading blocks**, or **Synced** with a Core-style status line.
- IBD live stats when catching up: blocks behind, blk/min, **estimated time left**, mempool txs, workers, assist pool.
- DGR (DogeGo relay CGNAT) strip when enabled.
- **P2P user-agent** line on Overview → Network (wire sub-version peers see, including optional tip address). When saved config differs from the live wire user-agent, a **restart pending** hint shows the value from `dogecoinconf.json`.
- **Addrbook strip** on Overview → Network: tried/new counts, **nKey** flag, and per-bucket max fill (from `GET /api/p2p`; mirrors **`getaddrmaninfo`** / **`dogego_addrbook_*`** RPC fields).
- **Operator certification (live)** on Overview → Network when Core RPC is configured or probe cache is warm: `N/17` live web gates, **Mempool corpus (offline) 58/58** + live parity when probes ran, cache age from `GET /api/core-status` (60s poll during IBD), **Run all probes** or **View cert matrix** (`#features/feat-cert-live-wrap`); click failed gate text to jump to the probe card.

### Sync
- Progress bar = block-body coverage vs header tip.
- Status line, ETA, contiguous height, orphan raw estimate, IBD worker stats, wallet rescan hints (`wallet_scan_index_ok`, `wallet_history_fast_path`, `wallet_listtransactions_utxo_walk`, `wallet_listtransactions_scan_pending`, `wallet_history_defer_reason`, `needs_rescan`), and addrbook tried/new strip fields (`addrbook_*`) from `GET /api/summary` (merged into Receive/Settings wallet stub and Overview P2P card before `/api/p2p` returns).
- **UTXO body replay** (`dogego_utxo_bodies_aligned`, `dogego_utxo_body_replay_remaining`, `dogego_snapshot_body_replay_pct`) on Overview → Sync, sync dock, and Features live strip when a UTXO snapshot is ahead of contiguous stored bodies.
- Chain search (height, hash, txid, address) - needs tx index for txs/addresses.
- Network, header tip, difficulty, wallet balance (when wallet enabled).
- Optional chain warnings banner.
- Quick links to Explorer, Mempool, Analytics, Features.

### Send
- Pay from built-in wallet (`sendtoaddress`) with optional fee/conf_target options.
- **Spendable balance** card (amount + optional pending / immature mining / UTXO meta chips).
- **Post-quantum (optional):** when **`pq_commitments`** is enabled (Settings → Wallet), Advanced exposes **PQ mode**: **OP_RETURN** (FLC1/DIL2/RCG4 tagged commitment on the send tx) or **carrier** (TX_C/TX_R P2SH pair via `dogego_sendpqcarrier`). Carrier mode requires **`pq_carrier_enabled`**. Raccoon-G (`RCG4`) is the Foundation [in-tree port](https://github.com/dogecoinfoundation/libdogecoin/tree/0.1.5-dev/src/raccoon_g) by [Ed Tubbs](https://github.com/edtubbs) ([@EdTubbs](https://x.com/EdTubbs); [Core PR #8](https://github.com/dogecoinfoundation/dogecoin/pull/8); [CREDITS.md](CREDITS.md)) linked in **GitHub Release** binaries via native CGO builds (see [pqcrypto/raccoon_g/BUILD.md](../pqcrypto/raccoon_g/BUILD.md)). Offline cert: **`dogego cert pq`**; live probe: **`GET /api/core-pq-probe`** (Features tab).
- **Coin control** (Advanced) loads `/api/wallet/utxos` from the in-memory UTXO cache when wired (fast on solo miners). Matches all wallet **SpendScripts** (HD receive/change). Shows immature coinbases disabled; lists first **300** UTXOs with a total count hint; surfaces unlock/fetch errors instead of hanging on “Loading…”.
- **Fee hints** on send failure - suggested feerate + one-click retry when the node rejects for insufficient fee.
- **Encrypted wallet** - when `wallet.json` is encrypted (setup wizard **Encrypt wallet on start** or Console `encryptwallet`), the node starts **locked** like Core. Click **Unlock & send** to open a passphrase dialog (`POST /api/wallet/unlock` → Core `walletpassphrase`), or unlock from CLI/RPC before spending.

### Receive
- Default receive address and QR-style display.
- Copy address; uses HD keypool.
- **Address book** tab - all tracked HD/import/watch/cosigner addresses sorted by **path / type** (receive indices, then **node tip** `m/44'/3'/0'/2/0`, then change, then imports); Core wallet.dat replay rows show `iskeypool` and `hd_keypool_core_index` as keypool/core-pool tags.
- Toolbar: **filter** (address, path, label, keypool/Core pool index), **Address type** (including keypool), **Generate address** (`POST /api/wallet/address/new` → Core `getnewaddress`), **Refill keypool** on HD wallets (`POST /api/wallet/keypool-refill` → Core `keypoolrefill`; useful after Core `wallet.dat` import when pool-only rows remain).
- Per-row **copy** button and inline **label edit** (`POST /api/wallet/address/label` → Core `setlabel`).
- **Monthly backup reminder** - banner when no wallet backup download in 30 days (browser `localStorage` **`dogego_backup_last_download`**); **Dismiss 30 days** snoozes the banner; download records a fresh timestamp.
- Stable refresh - address list reloads only when you open the tab or after generate/import/label (not on every wallet poll).
- Wallet path line (`#recv-meta`) and balance/status line (`#recv-status`) update independently to avoid layout jumps during sync.

### History
- Wallet transactions as a **card feed** with type chips (Sent, Received, Mining, Quantum).
- Balance and history use the **UTXO-cache fast path** when the node is wired (same source as BlockStep). **`GET /api/wallet`** also shows **`pq_commitments_enabled`**, **`pq_carrier_enabled`**, **`keypool_size`**, and Core pool index counts from `wallet.json`. Settings → **Wallet** toggles **`pq_commitments`** and **`pq_carrier`** live (`POST /api/wallet/flags`). History **Quantum** filter classifies sends with FLC1/DIL2/RCG4 OP_RETURN or **carrier** (`carrier_scriptsig` / TX_C reveal) on tx hex.
- **Infinite scroll** - loads **40 rows at a time** via `GET /api/wallet/txs?limit=40&offset=0` (scroll for more; stable scroll container).
- **Load all remaining** button when more rows exist; histories **≤200 txs auto-load fully** on tab open.
- **Search** (address, txid, label, …) and **type filters** are server-side (`q`, `kind` query params) with 300ms debounce.
- Compact **confirmation count** badge; **full txid** on each row; click a row for detail sheet (hero copy rows for address/txid, **BlockStep** / block / address actions).
- **Soft refresh** while scrolled deep updates confirmation badges without resetting scroll (paused when **`dogego_connect_lag` > 64** or IBD active).
- **Export CSV** respects active search/type filters via `GET /api/wallet/txs.csv?q=…&kind=…` (loopback).
- Rows include `tx_kind`, `pq_tag`, and carrier hints when the node can classify them (mining coinbase, PQ OP_RETURN or carrier sends).

### BlockStep
- Interactive block timeline; **hero copy rows** for transaction id and address (large centered text, copy on the side).
- **Wallet-owned addresses** use UTXO cache + `wallet.db` fast path (no 8000-block raw walk on solo miners).
- **PQ panels** on transaction detail: OP_RETURN commitment chips (FLC1/DIL2/RCG4) and TX_C/TX_R **carrier** banners with linked reveal tx when present (`/api/blockstep/tx`).
- 45s fetch timeout on `/api/blockstep/*` requests; **90s** on address sniff.

### Dashboard responsiveness during connect
- When **`dogego_connect_lag` > 32**, wallet balance polls defer; poll interval stretches to 2.5s (4s when lag > 128).
- Heavy wallet APIs (history pages, coin control list) defer when lag **> 64**, IBD is active, or **`wallet_listtransactions_scan_pending`** is set with >64 UTXOs (scan building index) unless you are on the relevant tab; **`GET /api/wallet`**, **`GET /api/summary`**, and **`GET /api/wallet/txs`** expose matching **`wallet_history_defer_reason`** / **`defer_reason`**; History reloads automatically when scan defer clears.
- Background slow polls (chainstats, mining, analytics) skip on inactive tabs during high connect lag.

### Explorer (full node)
- Search blocks, transactions, addresses stored locally.
- Depends on **tx index** (disable with `no_tx_index` only if you accept limited explorer).

### Mempool (full node)
- Pool size, min relay vs effective mempool min fee, feefilter aggregate.
- Package / standardness policy summary.
- Pause/resume mempool admission.
- Recent tx table (verbose when prevouts resolve).

### Analytics (full node)
- KPI row: block tip, sync %, stored blocks, network, chain disk + **disk breakdown** chips, hashrate, mempool, tx processed, 24h minted scan.
- **Connected peers** table: inbound/outbound `getpeerinfo` rows with sync role, ping, traffic, user-agent, and flags (DGR relay `NODE_DOGEGO_RELAY_CGNAT`, addnode, BIP152 HB, block score). Summary strip shows P2P mode and whether this node is using the QUIC DGR tunnel (`GET /api/peers`).
- Charts: sync progress, disk growth, mempool size, block sizes, miner distribution, header timing.
- **Top UTXO holders** table with ranked address pills (BlockStep + copy).
- Requires **embedded analytics sidecar** (setup or Settings).
- Read-only analytics APIs (`GET /api/analytics/summary`, `GET /api/analytics/metrics.csv`) follow the same reachability as `/api/summary` when `webui` binds beyond loopback (e.g. LAN dashboard on `0.0.0.0:2013`). With **`webui_remote_auth`** + a dashboard PIN, non-loopback clients must unlock before read APIs (Overview, Analytics, Mempool, …). Wallet and config POST routes remain loopback-only.

### Features & Core parity
The **Features** tab is the operator-facing Core integration dashboard. Data from `GET /api/capabilities`:

| Section | Purpose |
|---------|---------|
| **Core parity at a glance** | `parity_summary` - standalone exit gate, live/partial capability counts, open/partial/declined gaps (incl. `protocol_lock`), roadmap progress, RPC class counts |
| **When to try DogeGo** | `core_guidance` - beta testing fit, intentional differences (mainnet protocol lock), links to docs |
| **Certification** | `certification` from `GET /api/core-cert` - milestones A-E, offline harness (`dogego cert offline`, `cert_offline_prerequisites`, …), corpus sizes |
| **Status legend** | live / partial / planned / declined pills |
| **This node right now** | `live` runtime flags (P2P, wallet, indexes, DGR, relay policy) |
| **Roadmap highlights** | Checked vs open milestones from `ROADMAP.md` |
| **Core parity backlog** | Structured `core_parity_gaps` (sync with [CORE_PARITY_GAPS.md](CORE_PARITY_GAPS.md)) |
| **Capability categories** | Grouped features with **Open** to the relevant tab |
| **Live Core probes** | Mini-pill strip (compare, maintenance, restart-resume, **autostart**, **founder**, **runner**, connect lag, mempool, wallet, reindex, **BIP152**, **PQ**, end-to-end) - **click a pill** to jump to its probe card; **Run all probes** hits `GET /api/core-operator-cert?refresh=1`. Solo testnet: Core compare optional; IBD/sync shows **Syncing (checks OK)** on maintenance/reindex. |
| **Operator cert matrix** | seventeen live web gates (incl. Milestone D setup-parity, mining GBT/aux, PQ format probe, IBD convergence snapshot) + script-only soak rows; pill shows **Solo gates OK** when optional Core rows pass but strict Milestone E count does not; detail line shows `solo N/17`; **View probe** jumps to `#features/<card-id>` |
| **JSON-RPC commands** | Searchable method table (live / partial / stub) |

See also [CORE_PARITY_GAPS.md](CORE_PARITY_GAPS.md) § Web dashboard mapping.

### Docs
- Merged documentation hub from `GET /api/docs`: dashboard concepts (sync, P2P, mempool, wallet, Core parity), integration guides, operator runbooks, and embedded markdown viewer for repo `docs/*.md`.
- Search filters sections; click repo file links to render full markdown in-tab.
- Complements **Features** (live RPC table + parity counts).

### Console
- JSON-RPC console and rolling node log (`GET /api/log`).
- **RPC tutorial:** [RPC_CONSOLE_TUTORIAL.md](RPC_CONSOLE_TUTORIAL.md) (open from Console tab or Docs); **Method cookbook** loads `GET /api/rpc/cookbook` (all methods with curl + CLI; click **Use in Console**).
- **HTML reference:** `GET /api/rpc/reference.html` (all methods, help text, curl).
- **Core probe presets** (`GET /api/core-probes`, `/api/core-operator-cert`, individual probes, `/api/core-status`) - results appear in the RPC output panel and **sync the Features tab** probe cards in place.

### Settings
- Edits `dogecoinconf.json` for the **next** run (or **Restart node** for live service toggles).
- **Services (this run)** card at the top: contextual start/stop/restart for **solo mining**, **mempool relay**, **analytics sidecar**, and **node process** (`GET/POST /api/services`, **`POST /api/control/restart`**). Each row shows only the action that applies (Start when stopped, Stop when running). P2P and JSON-RPC are status-only (restart node to change). Mining controls apply to **this run**; Wallet tab **mine** checkbox persists after save+restart.
- **Tools** tab - searchable RPC catalog from **`GET /api/rpc/cookbook`** (`st-tools-groups`): Run in-process or **Open in Console** without leaving Settings.
- **Extensions** (sidebar) - GitHub extension catalog (`GET /api/extensions/catalog`), catalog sources, developer manual, enable/disable, install (catalog id or zip upload), uninstall. Each enabled extension opens a dedicated detail page; extensions with `ui_panel` show a dashboard from the extension status RPC (`GET /api/extensions/panel?id=…`), not host locale files. See [EXTENSIONS.md](EXTENSIONS.md).
- **Interface → Updates** - running version, last GitHub check, **Check now** (`POST /api/update/check`), download, install, dismiss (`st-update-status`).
- Sections: paths, **Wallet** (built-in wallet enable/`nowallet`, encrypt/change passphrase via **`POST /api/wallet/encrypt`** / **`POST /api/wallet/passphrase-change`**, lock/unlock spend keys, **`pq_commitments`** + **`pq_carrier`** toggles via **`GET/POST /api/wallet/flags`**, avoid_reuse, rescan), sync, analytics, P2P (**LAN peer pairing** card - share target + one-click `addnode`), **DogeGo relay (CGNAT)**, mining (intent only), interface (display/PIN), RPC & dashboard, advanced mempool/RPC auth, **external signer** (`signer_cmd` + **Test external signer** button).
- **Browser display** (poll interval, compact layout, show raw summary JSON, P2P JSON, language) - locale in `localStorage`; other prefs in `localStorage` only, not on disk.

## HTTP APIs used by the UI

### Docs tab

- **Sections** (`GET /api/docs`): audience-based index (Start here, sync, P2P, wallet, operator runbooks, integration, RPC, contributors).
- **Embedded markdown** (`GET /api/docs/md?path=…`): renders repo `docs/*.md` and module-root `ROADMAP.md` when available beside `go.mod`.
- **In-document links**: relative `.md` links and `#anchors` open inside the viewer (`GET /api/docs/resolve?base=…&href=…`). External `http(s)` links open in a new tab.
- **Navigation**: Back returns to the previous doc in the session; Close hides the viewer.

| Endpoint | Use |
|----------|-----|
| `GET /api/docs` | Docs tab (dashboard concepts + operator/integrator topics + embedded markdown) |
| `GET /api/rpc/cookbook` | All JSON-RPC methods with curl + CLI examples (Console cookbook browser) |
| `GET /api/rpc/reference.html` | HTML RPC reference (method, help, curl) |
| `GET /api/docs/md?path=…` | Embedded markdown viewer (JSON: `path`, `markdown`; 404 JSON includes `hint`) |
| `GET /api/docs/resolve?base=…&href=…` | Resolve a markdown link relative to the open document |
| `GET /api/docs/files` | List of embedded markdown paths |
| `GET /api/guide` | Legacy alias - same guide sections are merged into `/api/docs` |
| `GET /api/summary` | Overview, periodic poll (wallet rescan / scan-index hints when wallet enabled) |
| `GET /api/p2p` | Overview → Network P2P card (peers, addrbook tried/new, **`addrman_info`**, BIP152 HB, IBD assist) |
| `GET /api/analytics/summary` | Analytics tab (KPIs, chainstats, metric timeline, reorg events/summary, top UTXO holders; `?light=1` during IBD) |
| `GET /api/analytics/metrics.csv` | Analytics timeline CSV export |
| `GET /api/sync` | Sync bars and IBD stats |
| `GET /api/mempool` | Mempool tab |
| `GET /api/dgr` | DGR relay metrics |
| `GET /api/peers` | Analytics peer table - `getpeerinfo` rows + P2P/DGR context |
| `GET /api/lan-peer-hint` | Settings P2P LAN pairing - local private IPv4 + shareable `host:port` targets |
| `GET /api/explorer/...` | Search and explorer detail |
| `GET /api/wallet/...` | Balance, send, receive, txs - **UTXO-cache fast path** when wired; **`encrypted`**, **`unlocked`**, **`private_keys_enabled`**, **`unlocked_until`** on GET `/api/wallet` when the wallet file is encrypted. **`pq_commitments_enabled`** and **`pq_carrier_enabled`** on GET `/api/wallet`; **`GET/POST /api/wallet/flags`** toggles **`pq_commitments_enabled`** / **`pq_carrier_enabled`** (mirrors `setwalletflag`). **`POST /api/wallet/unlock`** (`passphrase`, optional `timeout` seconds, default 600) and **`POST /api/wallet/lock`** mirror Core **`walletpassphrase`** / **`walletlock`** (loopback + dashboard PIN gate). Send returns code **-13** with **`wallet_locked`** when spend keys are not loaded; carrier send with **`pq_mode: "carrier"`** returns **-8** when **`pq_carrier_enabled`** is false. **`keypool_size`**, **`change_keypool_size`**, **`pool_core_indices_stored`**, **`hd_keypool_core_index`**, **`hd_wallet`** from `wallet.json` on GET `/api/wallet` (Receive tab meta; no slow getwalletinfo). **`wallet_index_height`**, **`needs_rescan`**, **`wallet_scan_index_ok`**, **`wallet_history_fast_path`**, **`wallet_listtransactions_utxo_walk`**, **`wallet_listtransactions_scan_pending`**, **`scanning`** on GET `/api/wallet` and **`GET /api/summary`**; **`POST /api/wallet/rescan`** (incremental or `{full:true}`) backfills fee/hex in `wallet.db`; dashboard auto-starts incremental rescan once per browser session when caught up and (**`needs_rescan`** or **`wallet_listtransactions_utxo_walk`** with >64 UTXOs). History defers heavy **`listtransactions`** while **`wallet_listtransactions_scan_pending`** (>64 UTXOs) and reloads when the scan index is ready. Paginated `/api/wallet/txs?limit&offset&q&kind` returns **`deferred`** + **`defer_reason`** during IBD/connect lag/scan build; `/api/wallet/txs.csv` filtered export; `/api/wallet/utxos` for Send coin control; `/api/wallet/addresses` + `/api/wallet/labels` + `/api/wallet/address/new` + `/api/wallet/address/label` + **`POST /api/wallet/keypool-refill`** for Receive address book (`iskeypool` / `hd_keypool_core_index` when stored) |
| `GET /api/autostart` | OS login autostart status + verify (`ok`, `verify` vs `dogecoinconf.json`) |
| `GET /api/core-autostart-probe` | Operator cert gate for autostart=login (same logic as `dogego cert autostart`) |
| `GET /api/capabilities` | **Features tab** (parity summary, guidance, certification, categories, gaps, RPC) |
| `GET /api/core-cert` | Certification milestones A/B/D/E + corpus stats |
| `GET /api/core-operator-cert` | Live operator certification matrix (web gates + script-only backlog). Response includes `solo_ok` / `solo_pass` for solo testnet (optional Core gates count as pass). `?matrix=1` = definitions only; `?refresh=1` = bypass 90s cache |
| `GET /api/core-status` | Cached operator cert (incl. solo metrics) + `mempool_offline_corpus` / `mempool_parity_*` + Core RPC config (no probe run); Overview polls every 60s during IBD; startup warms probe cache after ~8s |
| `GET /api/core-probes` | All live Core probes in one request (Features tab bundle) - compare, maintenance, restart-resume, **ibd_convergence**, **addrman**, autostart, founder, runner, **workflow10**, setup_parity, reindex, **BIP152**, **pq**, mempool (stateless + `stateful_live`), wallet, end-to-end |
| `GET /api/core-pq-probe` | PQ format/carrier offline probe (FLC1/DIL2/RCG4 OP_RETURN round-trip + TX_C/TX_R carrier pair; verifier-side only; Features **feat-core-pq** card) |
| `GET /api/core-bip152-probe` | BIP152 v1 HB negotiate (`getpeerinfo` `bip152_hb_to`/`bip152_hb_from`); mirrors `scripts/core_bip152_probe.ps1` |
| `GET /api/core-end-to-end-probe` | Bundled operator workflow steps (mirrors `scripts/core_end_to_end_workflow.ps1`, incl. **`offline_corpus`**, **`bip125_offline`**, **`mempool_parity`**, **`ibd_convergence`**, `protocol_lock` + `bip152_hb`) |
| `POST /api/core-test` | Quick Core JSON-RPC reachability from Settings form |
| `POST /api/signer-test` | Quick HWI external signer enumerate from Settings form (loopback only) |
| `GET /api/core-reindex-probe` | Reindex/index check-only probe (getrpcinfo + getindexinfo) |
| `GET /api/core-wallet-probe` | Wallet workflow probe (`getwalletinfo` incl. `spendable_utxo_count`, `hd_keypool_core_index` / `pool_core_indices_stored` when present, `getbalance`, `getnewaddress`, `validateaddress`, address book count + keypool/core-pool index counts from `dogego_listwalletaddresses`, **`validateaddress`/`getaddressinfo` `iskeypool` round-trip** on first keypool row when present, `setlabel` round-trip); optional **`DOGEGO_WALLET_DAT`** `dogego_probewalletdat` pool fields (`pool_keys_matched`/`pool_keys_unmatched`, `pool_unmatched_hint`, `pool_indices_replayed`, `keypool_hint`) |
| `GET /api/core-maintenance` | Milestone E maintenance probe (verifychain, indexes, chaintxstats) |
| `GET /api/core-restart-resume` | Milestone E restart resume (checkpoint vs contiguous, assist pool, **os_autostart** when `autostart=login`)
| `GET /api/core-mining-probe` | Mining GBT / aux templates (`getmininginfo`, `getblocktemplate`, `createauxblock` in aux era); **fourteenth** live operator-cert gate; mirrors `scripts/core_mining_workflow.ps1` |
| `GET /api/core-addrman-probe` | Partial Core addrman snapshot (`getaddrmaninfo` bucket stats + chaininfo cross-check); **seventeenth** live operator-cert gate; mirrors `scripts/core_addrman_workflow.ps1`
| `GET /api/core-ibd-convergence-probe` | IBD progress snapshot (RPC + web + disk); timed convergence window via `scripts/ibd_convergence_check.ps1`
| `GET /api/core-autostart-probe` | OS login autostart cert gate (`dogego cert autostart` equivalent)
| `GET /api/core-founder-probe` | Reboot testnet founder preflight (`dogego cert founder`; skipped OK on mainnet)
| `GET /api/core-runner-probes` | dogego-live CI runner readiness (`dogego cert weekly`; `cli_workflow10` hint; preflight notes **`wallet_dat_pool_unmatched_hint`** / **`wallet_dat_keypool_refill_size`** when `DOGEGO_WALLET_DAT` set) |
| `GET /api/core-workflow10-probe` | Workflow 10 preflight (`dogego cert workflow10 -skip-scripts`; `?skip_provision=1` `?mine_bootstrap=1`; also in **`GET /api/core-probes`** bundle) |
| `POST /api/config/uacomment-preview` | Live wire user-agent preview for Settings / setup wizard |
| `POST /api/setup/founder-preflight` | Setup wizard Finish step founder checks (testnet only)
| `GET /api/core-compare` | Live getblockchaininfo + verifychain + gettxoutsetinfo + getmempoolinfo (size + **fullrbf/minrelaytxfee/incrementalrelayfee** + dogego_package_policy note) + getnetworkinfo + getdeploymentinfo (buried/BIP9 active-state protocol-lock check) vs Dogecoin Core (`core_rpc_addr` in Settings or env) |
| `GET /api/mempool/parity-probe` | Live testmempoolaccept on stateless rows; **`offline_corpus`** (58) + `offline_stateful` + `stateful_live` gate summary; Core side-by-side when `core_rpc_addr` set |
| `GET /api/mempool/stateful-status` | Read-only Milestone D stateful offline corpus + live 24/24 reboottestnet gate hints (no RPC rows) |
| `GET /api/core-setup-parity` | Read-only reboottestnet setup parity (`dogego cert setup-parity`) |
| `GET /api/log` | Console activity log |
| `GET/POST /api/config` | Settings load/save |
| `GET/POST /api/services` | Settings **Services (this run)** - node, solo mining, P2P, JSON-RPC, mempool, analytics; POST `{service, action}` (`start`/`stop`/`restart`/`pause`/`resume`/`clear`) |
| `POST /api/control/restart` | Detached node restart (loopback; spawns replacement with `-waitpid`) |
| `GET /api/update/status` | GitHub release check snapshot (also merged into `/api/summary`) |
| `GET /api/extensions` | Installed extensions list |
| `GET /api/extensions/catalog` | Remote catalog merged with install state (`?refresh=1` force fetch) |
| `POST /api/extensions/enable` | Enable extension `{ "id" }` |
| `POST /api/extensions/disable` | Disable extension `{ "id" }` |
| `POST /api/extensions/install` | Install `{ "id" }` or `{ "url", "sha256"? }` or multipart zip field `zip` |
| `POST /api/extensions/uninstall` | Uninstall `{ "id", "remove_data" }` |
| `GET /api/extensions/panel?id=<extension-id>` | Extension status panel (invokes extension `ui.status_method` RPC; renders `ui.panel_title` / `ui.summary`) |
| `POST /api/update/check` | Force immediate GitHub release check (loopback) |
| `POST /api/update/download` | Download + SHA256-verify release binary to `<datadir>/updates/` |
| `POST /api/update/apply` | Install verified update and restart into new binary (loopback) |
| `POST /api/update/dismiss` | Hide update banner until a newer release appears |
| `POST /api/setup` | Setup wizard save / save+start; optional **`wallet_encrypt`** + **`wallet_passphrase`** encrypts **`wallet.json`** before first node start (Core **`encryptwallet`**) |
| `GET /locales/{lang}.json` | i18n strings |
| `GET /i18n.js` | i18n runtime |

## Security

- Bind to **127.0.0.1** unless you terminate TLS and authenticate at a reverse proxy ([OPERATOR.md](OPERATOR.md)).
- The UI has **no login** - anyone who can reach the port can control the node and testnet wallet.
- Do not port-forward the dashboard to the public internet without a proxy and auth.

## Dashboard resilience (local use)

- `/api/live` poll uses **30s** timeout, **45s** default for other API calls; overlapping refreshes are skipped.
- **Wallet tab** loads balance/txs on a **separate async poll** (does not block Overview refresh); prefers UTXO-cache endpoints when the node is wired.
- Transient API slowness shows a **retrying** warning; hard disconnect banner only after **15** consecutive failures (or when no recent success within 60s).
- Hard-refresh static assets after UI updates: **Ctrl+Shift+R** (browser cache).

## Related docs

- [OPERATOR.md](OPERATOR.md) - datadir, RPC, P2P, wallet encryption
- [CORE_PARITY_GAPS.md](CORE_PARITY_GAPS.md) - Core parity backlog + Features tab mapping
- [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md) - declined parity items
- [ROADMAP.md](../ROADMAP.md) - certification milestones
