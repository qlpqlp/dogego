# DogeGo JSON-RPC tutorial (Web UI Console + CLI)

This guide shows how to call **every** supported JSON-RPC method from the dashboard **Console**, from any shell (**curl** on Linux, macOS, and Windows), and where to find auto-generated examples for all methods. PowerShell helpers are optional on Windows.

## Three ways to call RPC

| Way | When to use | Auth |
|-----|-------------|------|
| **Console tab** | Quick tests on the machine running DogeGo | None (loopback `POST /api/rpc`, in-process) |
| **HTTP JSON-RPC** | Scripts, remote tools on `rpc` listen address | `rpc_cookie` or `rpc_user` / `rpc_password` |
| **Cookbook / reference** | Copy-paste examples for **all** methods | Same as HTTP |

The Console does **not** need HTTP RPC enabled. It calls the same `rpc.Dispatch` path as `dogego node`.

## Web UI Console (step by step)

1. Open the dashboard (default `http://localhost:2013`).
2. Go to **Console** (or **Overview → Open Console**).
3. Under **RPC**:
   - **Method** - RPC name, e.g. `getblockchaininfo`, `addnode`, `sendtoaddress`.
   - **Params** - JSON **array** of arguments (use `[]` when there are none).
4. Click **Run** (or **Ctrl+Enter** in the params box).
5. Read the JSON in the output panel. Errors show `"error": { "code", "message" }`.

### Console examples

**Chain tip (no params):**

- Method: `getblockchaininfo`
- Params: `[]`

**Add a LAN peer (reboot testnet P2P port 44556):**

- Method: `addnode`
- Params: `["192.168.1.50:44556", "add"]`

Commands: `add` (persistent, saved to `dogecoinconf.json`), `onetry` (dial once), `remove`.

**List peers:**

- Method: `getpeerinfo`
- Params: `[]`

**Wallet send (unlock PIN first if enabled):**

- Method: `sendtoaddress`
- Params: `["TADDRESS...", 10.0]`

**Help for one method:**

- Method: `help`
- Params: `["getpeerinfo"]`

### Console presets

Preset buttons above the method field fill common operator calls (`getblockchaininfo`, `verifychain`, mining, header recovery). Edit params before Run.

### Full method list in the UI

On the same **Console** tab:

- **Method cookbook** - searchable list from `GET /api/rpc/cookbook` (every `SupportedMethods()` entry with curl + CLI examples). Click **Use in Console** to fill the form.
- **HTML reference** - `GET /api/rpc/reference.html` (opens in a new tab).
- **Features** tab - searchable RPC table with help text from `help.go`.

## HTTP JSON-RPC (curl)

Default listen address is in `dogecoinconf.json` → `"rpc"` (reboot **testnet** often uses `127.0.0.1:44555`; mainnet-style default is `127.0.0.1:22555`).

```bash
curl -sS --user RPCUSER:RPCPASS \
  -H 'content-type: application/json' \
  --data-binary '{"jsonrpc":"1.0","id":"1","method":"getblockchaininfo","params":[]}' \
  http://127.0.0.1:44555/
```

With **cookie auth** (`"rpc_cookie": true`), read user:password from `<datadir>/<chain>/.cookie` and pass as Basic auth (same as Dogecoin Core).

**Batch:** POST a JSON **array** of request objects; receive an array of responses.

## PowerShell helper (optional, Windows)

On Windows you can source the helper instead of typing curl:

```powershell
. .\scripts\dogego_rpc.ps1
Invoke-DogeGoJsonRpc -Method getblockchaininfo
Invoke-DogeGoJsonRpc -Method addnode -Params @("10.35.221.86:44556", "add")
```

The script reads `dogecoinconf.json` from `%APPDATA%\DogeGo\` and uses the configured `rpc` URL and cookie when set. Linux/macOS: use **curl** or the Console (above).

## dogego-cli column in the cookbook

`GET /api/rpc/cookbook` includes a **CLI** column in **dogecoin-cli** style:

```text
dogego-cli getblockchaininfo
dogego-cli addnode "10.35.221.86:44556" "add"
```

DogeGo does not ship a separate `dogego-cli` binary today. Use:

- **Console** (above), or
- **curl** (any OS) / **Invoke-DogeGoJsonRpc** (Windows helper), or
- **dogecoin-cli** with `-rpcconnect` / `-rpcport` pointed at DogeGo when auth matches.

## Machine-readable catalogs

| URL | Format |
|-----|--------|
| `GET /api/rpc/cookbook` | JSON: all methods + curl + CLI strings |
| `GET /api/rpc/reference.html` | HTML table (method, help, curl) |
| `GET /api/openrpc.json` | OpenRPC document |
| `GET /api/integration/guides` | Language snippets (Go, Python, Node, Rust) |

Loopback only from the dashboard origin (same as Console).

## Common workflows

### P2P / peers (testnet)

| Goal | Method | Params |
|------|--------|--------|
| Peer count | `getconnectioncount` | `[]` |
| Peer details | `getpeerinfo` | `[]` |
| Network summary | `getnetworkinfo` | `[]` |
| Add persistent peer | `addnode` | `["HOST:44556", "add"]` |
| One-shot dial | `addnode` | `["HOST:44556", "onetry"]` |
| List configured addnodes | `getaddednodeinfo` | `[true]` |
| Disconnect | `disconnectnode` | `["HOST:44556"]` |

Use **LAN IP** for two PCs on the same router (not the public IP). P2P port is **44556** on reboot testnet; RPC is **44555**.

### Sync / chain

| Goal | Method | Params |
|------|--------|--------|
| Tip + IBD fields | `getblockchaininfo` | `[]` |
| Header recovery | `dogego_recoverheaders` | `[]` |
| Verify chain | `verifychain` | `[4]` |
| Block by hash | `getblock` | `["HASH", 1]` |

### Mempool

| Goal | Method | Params |
|------|--------|--------|
| Mempool size | `getmempoolinfo` | `[]` |
| Test accept | `testmempoolaccept` | `[["RAWHEX"]]` |
| Broadcast | `sendrawtransaction` | `["RAWHEX"]` |

### Wallet (built-in)

See [WALLET.md](WALLET.md). Examples: `getwalletinfo`, `getnewaddress`, `sendtoaddress`, `listtransactions`, `walletcreatefundedpsbt`.

Unlock wallet RPC when dashboard PIN is enabled (Console uses the same session after unlock).

### Mining (testnet)

| Goal | Method | Params |
|------|--------|--------|
| Mine blocks | `generatetoaddress` | `[1, "TADDRESS..."]` |
| Mining status | `getmininginfo` | `[]` |

Or set `"mine": true` in config for background mining on reboot testnet.

## Errors

| Code | Meaning |
|------|---------|
| `-32601` | Method not implemented |
| `-8` | Invalid parameters |
| `-31` | P2P disabled (e.g. `addnode` when P2P not wired) |
| Wallet errors | Unlock required, insufficient funds, etc. |

## Related docs

- [INTEGRATION.md](INTEGRATION.md) - auth, ports, dashboard `/api/*`
- [RPC.md](RPC.md) - workflow index by category
- [WEB_UI.md](WEB_UI.md) - Console tab and loopback APIs
- [OPERATOR.md](OPERATOR.md) - reboot testnet founder / `addnode` checklist
