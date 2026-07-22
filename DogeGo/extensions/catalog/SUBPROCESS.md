# Subprocess extensions

Third-party extensions can ship a **sandboxed subprocess** (`entry.type: subprocess`) instead of a builtin Go module.

## Layout

```
example.go/
  dogego.extension.json
  hello-ext          (or hello-ext.exe on Windows)
```

## Wire protocol

Line-delimited JSON (one request per line, one response per line). Max line 4 MiB.

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

## Lifecycle methods

| Method | When |
|--------|------|
| `dogego_on_enable` | After process start; params `[{ "protocol":1, "network":"testnet", "data_dir":"..." }]` |
| `dogego_on_disable` | Before SIGKILL on disable |

## Environment (allowlist)

| Variable | Meaning |
|----------|---------|
| `DOGEGO_EXT_ID` | Extension id |
| `DOGEGO_EXT_RPC` | Protocol version (`1`) |
| `DOGEGO_NETWORK` | `mainnet` / `testnet` |
| `DOGEGO_DATA_DIR` | Extension data directory only |

Plus `PATH`, `TEMP`, `HOME` / `USERPROFILE`, `SYSTEMROOT`. **No wallet variables.**

## DogeGo RPC surface

Inner methods map to:

`dogego_ext_<id_with_underscores>_<method>`

Built-in subprocess methods:

- `info` - runtime status (answered by host)
- `ping` - forwarded to subprocess (if implemented)

## Example

See [example-go/hello](example-go/hello/main.go). Build zip:

```bash
cd DogeGo/extensions/catalog/example-go
go build -o dist/hello-ext ./hello
(cd dist && zip -r hello.zip ../dogego.extension.json ../icon.png hello-ext)
dogego-cli dogego_instextensionzip dist/hello.zip
dogego-cli dogego_enableextension example.go
dogego-cli dogego_ext_example_go_ping
```

## Security

- Binary must live inside the extension install dir (no `..` in `entry.binary`).
- Subprocess cannot access wallet APIs (not exposed over this protocol).
- Host validates `rpc_register` permission before forwarding RPC.

## Wasm

`entry.type: wasm` is supported. See [WASM_PROTOCOL.md](WASM_PROTOCOL.md). Use subprocess when you need OS APIs.
