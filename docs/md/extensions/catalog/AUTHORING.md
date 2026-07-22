# Authoring DogeGo extensions

Third-party extensions ship a **`dogego.extension.json`** manifest and optional assets. DogeGo loads them from catalog or zip install, then enable.

## Package layout

```
my.extension/
  dogego.extension.json
  README.md
  (wasm/subprocess assets when runtime ships)
```

## Manifest example

```json
{
  "manifest_version": 1,
  "id": "com.example.hello",
  "name": "Hello Extension",
  "version": "0.1.0",
  "author": "You",
  "description": "Example extension",
  "dogego_min_version": "0.1.0",
  "networks": ["testnet", "mainnet"],
  "permissions": ["chain_read", "rpc_register"],
  "entry": { "type": "wasm", "module": "com.example.hello", "wasm": "plugin.wasm" }
}
```

Optional: declare extension-owned RPC + help (shown in Console cookbook without editing DogeGo):

```json
{
  "permissions": ["rpc_register", "ui_panel"],
  "ui": { "status_method": "info" },
  "rpc": [
    { "name": "info", "help": "Status + Settings panel summary." },
    { "name": "ping", "help": "Health check." },
    { "name": "greet", "help": "Greet a name (params: [\"World\"])." }
  ]
}
```

See [HELLO_WORLD.md](HELLO_WORLD.md) for a full walkthrough (`example.go`).

**Packaging:** see [BUILDING.md](BUILDING.md) for wasm (universal), subprocess per-platform zips, universal fat zips (`entry.binaries`), and source-only installs.

## Manifest: platform-specific subprocess binaries

For a **universal zip** containing all OS builds, map platform keys to paths inside the archive:

```json
{
  "entry": {
    "type": "subprocess",
    "module": "com.example.hello",
    "binary": "hello-ext",
    "binaries": {
      "windows-amd64": "bin/windows-amd64/hello-ext.exe",
      "linux-amd64": "bin/linux-amd64/hello-ext",
      "darwin-arm64": "bin/darwin-arm64/hello-ext"
    }
  }
}
```

At install time DogeGo copies the matching file to `entry.binary` (e.g. `hello-ext` / `hello-ext.exe`). Keys use `goos-goarch` (see [BUILDING.md](BUILDING.md)).

## Permissions

| Permission | Grants |
|------------|--------|
| `chain_read` | Tip height, block hash, raw blocks, tx index lookups |
| `chain_index` | Confirmed tx position in block |
| `datadir_write` | `ExtensionDataDir(id)` under `<datadir>/extensions/<id>/data/` |
| `p2p_extension` | Overlay P2P after `exthello`/`extack` |
| `rpc_register` | `RPCProvider` methods under `dogego_ext_<id>_…` |
| `ui_panel` | Optional dashboard panel: status RPC returns a `ui` object. Prefer `layout: "workspace"` with `nav` + `sections` (home stays light; menu/tabs; optional `wizards`). Legacy `tools` / `widgets` / `quick_actions` still work. |
| `wallet_rpc` | Whitelisted wallet JSON-RPC (`signmessage`, `sendtoaddress`, `createrawtransaction`, `fundrawtransaction`, …). Spend/sign needs unlock via `walletpassphrase` in the dashboard (extensions cannot unlock themselves). Configure usage in the extension's own Settings section when provided. |

**Forbidden:** `wallet`, `private_keys`, `sign_message`, `sign_transaction`, `spend`

### Wallet RPC from extensions

Declare `wallet_rpc` only when your extension needs wallet operations. The host never exposes keys; it forwards **allowlisted** methods to the same JSON-RPC dispatch as the Console.

| Category | Examples | Unlock required |
|----------|----------|-----------------|
| Read-only | `getwalletinfo`, `listunspent`, `validateaddress` | No |
| Sign / send | `signmessage`, `sendtoaddress`, `signrawtransactionwithwallet`, `fundrawtransaction` | Yes (`walletpassphrase`) |
| Build / decode | `createrawtransaction`, `decoderawtransaction` | No |
| Broadcast | `sendrawtransaction` | No (signed hex only) |

**Never allowed from extensions:** `dumpprivkey`, `importprivkey`, `walletpassphrase`, `signmessagewithprivkey`, …

Go builtins call `host.(extensions.WalletRPCHost).CallWalletRPC(method, params)`.

Subprocess extensions use host RPC `wallet_call` with params `["method", …args]` (not forwarded to the binary).

## Extension icons

Ship a **PNG** at the package root (`icon.png`) or in a subfolder (e.g. `assets/icon.png`). Declare the relative path in manifest:

```json
{
  "icon": "icon.png"
}
```

The dashboard serves icons from the installed package (or catalog embed for built-ins) via `GET /api/extensions/icon?id=<extension-id>`. Do not embed extension icons in DogeGo core.

## Extension interfaces (Go builtin)

```go
type Extension interface {
    Manifest() Manifest
    OnEnable(ctx context.Context, host Host) error
    OnDisable() error
    HandleRPC(method string, params []json.RawMessage, host Host) (interface{}, error)
}
```

Optional:

- `RPCProvider` - advertise RPC + help
- `P2PExtension` - decentralized overlay protocol
- `BlockIndexExtension` - live L1 indexing on new blocks
- `PeerSyncExtension` - catch-up when peers negotiate overlays

## RPC naming

Inner method `ping` on extension `com.example.hello` becomes:

`dogego_ext_com_example_hello_ping`

## Dashboard panel (`ui_panel` permission)

Extensions add WebUI **only through a JSON panel** rendered by DogeGo. They cannot inject HTML/JS/CSS.

Full contract: **[UI_PANEL.md](UI_PANEL.md)**.

Declare `ui_panel` and a status RPC:

```json
{
  "permissions": ["rpc_register", "ui_panel"],
  "ui": { "status_method": "info" }
}
```

Preferred shape (modern menu + sections):

```json
{
  "ui": {
    "panel_title": "Hello status",
    "subtitle": "Running",
    "layout": "workspace",
    "nav": [
      { "id": "home", "label": "Home", "icon": "home" },
      { "id": "tools", "label": "Tools", "icon": "construction" },
      { "id": "settings", "label": "Settings", "icon": "tune" }
    ],
    "sections": {
      "home": { "title": "Overview", "lead": "Keep Home light.", "quick_actions": [{ "id": "refresh", "label": "Refresh", "method": "info", "icon": "refresh" }] },
      "tools": { "title": "Tools", "tools": [{ "id": "ping", "label": "Ping", "method": "ping", "icon": "network_ping" }] },
      "settings": { "title": "Settings", "lead": "Wallet RPC toggles and preferences belong here." }
    }
  }
}
```

Go: `extensions.DefaultWorkspaceUI("Hello", "subtitle", extensions.ToolsFromManifest(m))`.

The host **auto-wraps** older flat `tools` / `widgets` / `summary` panels into the same Menu shell, so every `ui_panel` extension gets Home / Tools / Settings navigation.

**Do not** add extension-specific strings to DogeGo locale files. Return copy from your extension.

HTTP: `GET /api/extensions/panel?id=com.example.hello` (enabled + `ui_panel`; dashboard auth applies).

## Publishing to the catalog

1. Add your entry to `extensions/catalog/catalog.json` (or host your own catalog JSON).
2. Choose a packaging strategy from [BUILDING.md](BUILDING.md):
   - **Wasm:** one `download_url` + `sha256` (all platforms).
   - **Subprocess prebuilt:** `downloads` map with one HTTPS zip per `goos-goarch` key.
   - **Subprocess universal:** one zip with `entry.binaries` in the manifest; catalog `downloads.universal`.
   - **Subprocess source:** `repository` (GitHub tree) or source zip; requires Go on the install host.
3. Pin `sha256` on every published zip when possible.

Operators install with:

```bash
dogego-cli dogego_instextension com.example.hello
dogego-cli dogego_instextension example.wasm
```

**Catalog `downloads` example:**

```json
{
  "id": "com.example.hello",
  "downloads": {
    "windows-amd64": { "download_url": "https://…/hello-windows-amd64.zip", "sha256": "…" },
    "linux-amd64": { "download_url": "https://…/hello-linux-amd64.zip", "sha256": "…" }
  }
}
```

DogeGo picks the entry matching the host platform at install time.

## Security checklist

- Prefer structured `ui` panels ([UI_PANEL.md](UI_PANEL.md)); never try to inject HTML/JS.
- Request `wallet_rpc` only when needed; never ask for raw key permissions (rejected at install).
- Validate all P2P and RPC input; reject oversize blobs.
- Use `datadir_write` only under your extension data dir.
- Pin `sha256` on catalog entries when possible.

## Entry types today

| Type | Install | Enable |
|------|---------|--------|
| `builtin` | Shipped in DogeGo binary | Yes |
| `subprocess` | Yes | Yes ([protocol](SUBPROCESS_PROTOCOL.md)) |
| `wasm` | Yes | Yes ([protocol](WASM_PROTOCOL.md)) |

See [SUBPROCESS_PROTOCOL.md](SUBPROCESS_PROTOCOL.md) for the line JSON-RPC binary protocol.

See [WASM_PROTOCOL.md](WASM_PROTOCOL.md) for the wasm export-per-RPC protocol.

See [EXTENSIONS.md](../EXTENSIONS.md) and the built-in [dogego.zkl2](zkl2/docs/USER_GUIDE.md) reference implementation.
