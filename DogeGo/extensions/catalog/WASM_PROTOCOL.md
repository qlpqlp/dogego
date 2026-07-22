# Wasm extensions (`entry.type: wasm`)

Third-party extensions can ship a **WebAssembly module** (`entry.wasm`) executed in-process by DogeGo via [wazero](https://wazero.io). No WASI or filesystem access is exposed to the guest by default.

## Manifest

```json
{
  "manifest_version": 1,
  "id": "example.wasm",
  "name": "Wasm Ping",
  "version": "0.1.0",
  "permissions": ["chain_read", "datadir_write", "rpc_register"],
  "networks": ["mainnet", "testnet"],
  "entry": {
    "type": "wasm",
    "module": "example.wasm",
    "wasm": "ping.wasm"
  }
}
```

`entry.wasm` must be a filename inside the install directory (no `..` or path separators).

## Lifecycle exports (optional)

| Export | Signature | When |
|--------|-----------|------|
| `dogego_on_enable` | `() -> ()` | After module instantiate |
| `dogego_on_disable` | `() -> ()` | Before module close |

## RPC dispatch

Inner method `ping` maps to RPC `dogego_ext_<id>_ping`.

The host looks up a wasm export with the **same name** as the inner RPC method (e.g. `ping`). If missing, it falls back to export `dogego_rpc`.

Built-in handlers (always available):

| Method | Behavior |
|--------|----------|
| `info` | Module path, version, `alive` |
| `ping` | Calls export `ping()` when present |

### Calling convention

| Guest export | Host behavior |
|--------------|---------------|
| `() -> i32` | Return value passed through (`ping` → `{"pong": N}` or `"pong"` when 0) |
| `() -> ()` | `null` result |
| `(i32, i32) -> i32` | First param is a guest pointer, second is length; host writes the first JSON RPC param string into guest memory (requires guest `malloc`) |

Guest modules may export `malloc` / `free` for string arguments. The host import module `dogego` exposes:

| Import | Signature | Purpose |
|--------|-----------|---------|
| `dogego.log` | `(ptr, len) -> ()` | Log UTF-8 text via extension `Host.Log` |

## Limits

| Limit | Value |
|-------|-------|
| Module size on disk | 16 MiB |
| Zip install (overall) | 64 MiB |
| Guest memory writes (log) | 4096 bytes max per call |

## Example

See [example-wasm/](example-wasm/). Build zip:

```powershell
cd DogeGo/extensions/catalog/example-wasm
./build.ps1
```

```bash
cd DogeGo/extensions/catalog/example-wasm
./build.sh
```

```bash
dogego-cli dogego_instextensionzip ping.zip
dogego-cli dogego_enableextension example.wasm
dogego-cli dogego_ext_example_wasm_ping
```

## Security

- No wallet, signing, or network syscalls in the wasm sandbox.
- `rpc_register` permission required for extension RPC forwarding.
- Prefer **subprocess** when you need OS APIs; wasm is for portable, sandboxed logic.

See [AUTHORING.md](AUTHORING.md) and [EXTENSIONS.md](../EXTENSIONS.md).
