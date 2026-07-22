# DogeGo Extensions

Optional **extensions** let operators add features without changing Dogecoin mainnet consensus. Extensions cannot access private keys directly; optional wallet operations use a **whitelisted wallet RPC bridge** when `wallet_rpc` is declared in the manifest.

Extension lifecycle: **browse catalog → install → enable → disable → uninstall**.

## Security model

| Rule | Detail |
|------|--------|
| **No consensus fork** | Extensions run off-chain beside the full node |
| **No direct key access** | Forbidden permissions: `wallet`, `private_keys`, `sign_message`, `sign_transaction`, `spend` |
| **Allowlisted permissions** | `chain_read`, `chain_index`, `datadir_write`, `p2p_extension`, `rpc_register`, `ui_panel`, `wallet_rpc` (enforced at runtime via scoped host) |
| **Install verify** | Every package must ship `dogego.extension.json` (schema v1); zip path traversal blocked; HTTPS-only remote installs |
| **SHA256 optional** | Catalog entries may pin `sha256` of release zip |
| **Wallet via RPC** | With `wallet_rpc`, extensions call whitelisted wallet methods only. Unlock with `walletpassphrase` before sign/send |
| **Extension icons** | PNG in the extension package (`icon.png` or subfolder); served via `GET /api/extensions/icon?id=…` |

## Install sources

1. **Built-in** - shipped inside the binary (`entry.type: builtin`)
2. **GitHub catalog** - `extensions/catalog/catalog.json` (fetched from GitHub raw URL)
3. **Zip file** - local path or dashboard upload (`dogego_instextensionzip` / Extensions → Install zip)
4. **HTTPS URL** - `dogego_instextensionurl` with optional SHA256

Installed files live under `<datadir>/<network>/extensions/<id>/`.

## Remote catalog

Default catalog URL:

`https://raw.githubusercontent.com/qlpqlp/dogego/main/DogeGo/extensions/catalog/catalog.json`

Package layout and cross-platform build rules: [extensions/catalog/README.md](../extensions/catalog/README.md) and [extensions/catalog/BUILDING.md](../extensions/catalog/BUILDING.md).

Schema (`catalog_version: 1`):

```json
{
  "catalog_version": 1,
  "updated": "2026-07-13",
  "extensions": [{
    "id": "example.go",
    "name": "Hello World Extension",
    "version": "0.2.0",
    "downloads": {
      "windows-amd64": { "download_url": "https://…/hello-windows-amd64.zip", "sha256": "…" },
      "linux-amd64": { "download_url": "https://…/hello-linux-amd64.zip", "sha256": "…" }
    },
    "repository": "https://github.com/…/tree/main/…/extensions/catalog/example-go"
  }]
}
```

Platform keys are `goos-goarch` (e.g. `windows-amd64`). DogeGo picks the matching `downloads` entry at install time. Use `downloads.universal` for one fat zip whose manifest sets `entry.binaries`. Wasm extensions use a single `download_url` (portable). See [BUILDING.md](../extensions/catalog/BUILDING.md).

Cached under `<datadir>/extensions/catalog.cache.json` (6h TTL).

## Manage (RPC)

| RPC | Purpose |
|-----|---------|
| `dogego_listextensions` | List built-in + installed extensions |
| `dogego_listextensioncatalog` | Remote catalog merged with local state (`true` = force refresh) |
| `dogego_enableextension "id"` | Enable (starts sandbox host) |
| `dogego_disableextension "id"` | Disable |
| `dogego_instextensionzip "path"` | Install from local zip |
| `dogego_instextensionurl "url" ["sha256"]` | Install from HTTPS zip |
| `dogego_instextension "id"` | Install from catalog entry |
| `dogego_uninstextension "id" [remove_data]` | Uninstall non-builtin package |

Extension-owned RPCs use prefix `dogego_ext_<id_with_underscores>_<method>`.

## Web UI

**Extensions** (sidebar)

- Browse catalog (GitHub JSON)
- Enable / Disable / Install / Uninstall
- Upload zip (multipart, 64 MiB max, path-traversal safe extract)
- Per-extension detail page for enabled extensions with `ui_panel` permission (copy comes from the extension status RPC, not host locales)

HTTP API (dashboard PIN when configured):

| Route | Method |
|-------|--------|
| `/api/extensions` | GET list installed |
| `/api/extensions/catalog` | GET catalog (`?refresh=1`) |
| `/api/extensions/enable` | POST `{ "id" }` |
| `/api/extensions/disable` | POST `{ "id" }` |
| `/api/extensions/install` | POST `{ "id" }` or `{ "url", "sha256"? }` or multipart zip |
| `/api/extensions/uninstall` | POST `{ "id", "remove_data" }` |
| `/api/extensions/panel?id=<extension-id>` | GET extension status panel (invokes extension `ui.status_method` RPC) |
| `/api/extensions/docs?path=` or `?id=` | GET extension markdown docs |

## Manifest (`dogego.extension.json`)

See [example manifest](../extensions/catalog/example-go/dogego.extension.json).

```json
{
  "manifest_version": 1,
  "id": "dogego.zkl2",
  "name": "DogeGo ZK Layer 2",
  "version": "0.1.0",
  "permissions": ["chain_read", "chain_index", "datadir_write", "rpc_register", "p2p_extension", "ui_panel", "wallet_rpc"],
  "ui": { "status_method": "info" },
  "networks": ["mainnet", "testnet"],
  "entry": { "type": "builtin", "module": "dogego.zkl2" }
}
```

**Entry types**

| Type | Status |
|------|--------|
| `builtin` | Supported (first-party modules compiled into DogeGo) |
| `subprocess` | Supported ([subprocess protocol](../extensions/catalog/SUBPROCESS_PROTOCOL.md)) |
| `wasm` | Supported ([wasm protocol](../extensions/catalog/WASM_PROTOCOL.md)) |

Example subprocess package: build `hello.zip` on your OS ([example-go docs](../extensions/catalog/example-go/docs/README.md)). Walkthrough: [HELLO_WORLD.md](../extensions/catalog/HELLO_WORLD.md).

Example wasm zip: `extensions/catalog/example-wasm/ping.zip` (portable wasm; build with `./build.ps1` or `./build.sh` in `example-wasm/`).

## Research: `dogego.bbpow` (testnet only)

Beta **Bitcoin-Backed PoW (BBPoW / CAuxPoW)** verifier: SHA-256 Bitcoin commitments as a research Dogecoin security signal. **Not AuxPoW** (different algorithms). **Not L1 consensus** (would be a hard fork if adopted). Package: [extensions/catalog/bbpow/](../extensions/catalog/bbpow/). Build zip with `build.ps1` / `build.sh`, install on **testnet**, then enable.

```bash
dogego-cli dogego_instextensionzip path/to/bbpow.zip
dogego-cli dogego_enableextension dogego.bbpow
dogego-cli dogego_ext_dogego_bbpow_compare
```

Docs: [BBPoW user guide](../extensions/catalog/bbpow/docs/USER_GUIDE.md), [BBPoW protocol sketch](../extensions/catalog/bbpow/docs/PROTOCOL.md).

## Catalog: `dogego.doginals`

Beta **Doginals / DRC-20 L2** - same overlay style as ZK L2 (`exthello`/`extack`), protocol id **`doginals-v1`**.

- **L1**: observe-only index of OP_RETURN / data-carrier outputs + **DRC-20 token summaries**
- **Mint**: deploy / mint / transfer wizard builds OP_RETURN JSON; optional broadcast via **authenticated `wallet_rpc`** (enable in extension Settings; unlock wallet in dashboard)
- **L2**: off-chain NFT / token / image metadata; P2P `dinv` / `getdasset` / `dasset`
- **UI**: modern workspace (Home + menu sections + wizards)
- **Networks**: mainnet + testnet · **no consensus change**

Package: [extensions/catalog/doginals/](../extensions/catalog/doginals/). Install zip then enable.

| RPC | Purpose |
|-----|---------|
| `dogego_ext_dogego_doginals_info` | Status + UI workspace |
| `dogego_ext_dogego_doginals_listtokens` / `gettoken` / `listbytick` | DRC-20 token index |
| `dogego_ext_dogego_doginals_previewinscription` / `inscribe` | Build / broadcast DRC-20 OP_RETURN |
| `dogego_ext_dogego_doginals_indexrange` | Scan L1 height window |
| `dogego_ext_dogego_doginals_putasset` / `getasset` / `listassets` | L2 assets |
| `dogego_ext_dogego_doginals_getconfig` / `setconfig` | Extension Settings (wallet RPC toggle) |

Docs: [USER_GUIDE](../extensions/catalog/doginals/docs/USER_GUIDE.md), [PROTOCOL](../extensions/catalog/doginals/docs/PROTOCOL.md).

```bash
dogego-cli dogego_instextensionzip path/to/doginals.zip
dogego-cli dogego_enableextension dogego.doginals
```

## Built-in: `dogego.zkl2`

Optional ZK L2 inspired by [Dogecoin discussion #3869](https://github.com/dogecoin/dogecoin/discussions/3869) - **without** `OP_CHECKZKP` or any protocol fork.

**Install first** (catalog source or zip), then enable. Package lives under [extensions/catalog/zkl2/](../extensions/catalog/zkl2/) (manifest, docs, icon, Go sources). The DogeGo binary links the module for P2P overlay and block indexing.

- **Decentralized sync**: `exthello`/`extack` then `zkproof-v1` overlay (`zkinv`, `getzkproof`, `zkproof`, `getzkheaders`, `zkheaders`)
- L1 anchor: optional `OP_RETURN` tag **`ZKDG`** (recognition-only on L1)
- L2 state: Pebble DB under `extensions/dogego.zkl2/data/`
- Groth16 verify off-L1: compressed 192 B + DIP 384 B proofs; VK from `data/vk/default.vk` or inline `verifying_key` / `verifying_key_chunks` (6×80 B)

| RPC | Purpose |
|-----|---------|
| `dogego_ext_dogego_zkl2_info` | Extension + P2P status |
| `dogego_ext_dogego_zkl2_submitproof` | Submit tx-anchored proof (relays `zkinv`) |
| `dogego_ext_dogego_zkl2_verifyproof` | Verify without storing (optional `verifying_key` or `verifying_key_chunks` for #3869 inline VK) |
| `dogego_ext_dogego_zkl2_checkzkp` | Alias for verifyproof |
| `dogego_ext_dogego_zkl2_getproof` / `listproofs` / `proofroot` | Query local index |
| `dogego_ext_dogego_zkl2_prepareanchor` | Optional ZKDG anchor + signmessage payload |
| `dogego_ext_dogego_zkl2_signanchor` | Prepare anchor and sign via `wallet_rpc` (wallet must be unlocked) |

Docs:

- [ZK protocol](../extensions/catalog/zkl2/docs/PROTOCOL.md)
- [ZK user guide - submit proofs & sync](../extensions/catalog/zkl2/docs/USER_GUIDE.md)

Enable:

```bash
dogego-cli dogego_instextension dogego.zkl2
dogego-cli dogego_enableextension dogego.zkl2
```

## Authoring extensions

Implement `extensions.Extension`. Optionally:

- `RPCProvider` - advertise `RPCMethods()`
- `P2PExtension` - `ProtocolID()`, `P2PCommands()`, `HandleP2P()`
- `BlockIndexExtension` - `OnBlockConnected()` for live L1 indexing
- `PeerSyncExtension` - `OnPeerConnected()` for overlay catch-up

See [Authoring extensions](../extensions/catalog/AUTHORING.md).

## Repository layout

| Path | Purpose |
|------|---------|
| `extensions/catalog/` | Official catalog JSON, example packages, packaging docs |
| `extensions/zkl2/` | Built-in ZK L2 Go module |
| `extensions/*.go` | Extension host (install, registry, subprocess, wasm) |
| `docs/EXTENSIONS.md` | This operator overview |
