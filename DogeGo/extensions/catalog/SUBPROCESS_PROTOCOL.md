# Subprocess extensions (`entry.type: subprocess`)

Third-party extensions can ship a **native binary** executed by DogeGo in a sandbox (no wallet env, extension datadir only).

## Binary protocol (line JSON-RPC)

stdin/stdout, one JSON object per line, UTF-8.

**Request:**

```json
{"id":1,"method":"ping","params":[]}
```

**Response:**

```json
{"id":1,"result":"pong"}
```

or

```json
{"id":1,"error":"message"}
```

### Lifecycle methods (required)

| Method | When | Params |
|--------|------|--------|
| `dogego_on_enable` | After start | `[{"protocol":1,"network":"testnet","data_dir":"..."}]` |
| `dogego_on_disable` | Before kill | `[]` |

### Environment (safe subset)

| Variable | Meaning |
|----------|---------|
| `DOGEGO_EXT_ID` | Extension id |
| `DOGEGO_EXT_RPC` | Protocol version (`1`) |
| `DOGEGO_NETWORK` | `mainnet` / `testnet` |
| `DOGEGO_DATA_DIR` | Writable extension data path |

Wallet and private-key environment variables are **not** passed.

## Manifest

```json
{
  "entry": {
    "type": "subprocess",
    "module": "example.go",
    "binary": "hello-ext"
  },
  "permissions": ["rpc_register", "datadir_write", "chain_read"]
}
```

On Windows, DogeGo tries `hello-ext` then `hello-ext.exe`.

## Public RPC

Declare methods in manifest `rpc` (name + help). They appear in Console cookbook and `dogego_ext_<id>_*` help without editing DogeGo core.

Inner method `ping` becomes `dogego_ext_example_hello_ping`.

Host handlers (subprocess runtime):

- `info` - subprocess status (pid, alive); merges `ui` from binary `ui_status` when `ui_panel` permission is set
- `chain_tip` - Dogecoin tip height when `chain_read` is granted (answered by host, not forwarded to binary)
- `wallet_call` - whitelisted wallet JSON-RPC when `wallet_rpc` is granted; params `["method", ...args]` (host only, not forwarded to binary)
- other manifest `rpc` names - forwarded to the binary stdin/stdout protocol

Binary-only inner method (not in public manifest):

- `ui_status` / `info` - returns `{ "ui": { … } }` for the extension workspace panel (see [UI_PANEL.md](UI_PANEL.md); prefer `layout: "workspace"` with `nav` + `sections`)

Full walkthrough: [HELLO_WORLD.md](HELLO_WORLD.md).

## Example

See [example-go/hello](example-go/hello/main.go). Build release zip:

```powershell
cd DogeGo/extensions/catalog/example-go
./build.ps1
```

```bash
cd DogeGo/extensions/catalog/example-go
./build.sh
```

Then install:

```bash
dogego-cli dogego_instextensionzip hello.zip
dogego-cli dogego_enableextension example.go
dogego-cli dogego_ext_example_hello_ping
```

The Windows `hello-ext.exe` is ~2 MiB (static Go binary). Rebuild locally with `build.ps1` / `build.sh` before publishing; update `catalog.json` `sha256` after each release zip change.

## Wasm

`entry.type: wasm` is supported. See [WASM_PROTOCOL.md](WASM_PROTOCOL.md). Prefer **subprocess** when you need OS APIs.
