# DogeGo JSON-RPC guide

**Live catalog:** run the node and open the dashboard **Features** tab (searchable table from `SupportedMethods()` + `help.go`), or call RPC `help`.

**Specification:** [Dogecoin Core RPC](https://github.com/dogecoin/dogecoin/blob/master/doc/bitcoin-api.md) is the behavioral reference; DogeGo implements a **subset** with documented gaps ([INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md), [ROADMAP.md](../ROADMAP.md)).

## Connection

See [INTEGRATION.md](INTEGRATION.md) for HTTP auth, ports, batching, and client examples.

**Tutorial (Web UI Console + curl + PowerShell):** [RPC_CONSOLE_TUTORIAL.md](RPC_CONSOLE_TUTORIAL.md) - step-by-step Console usage, LAN `addnode`, and links to the full method cookbook (`GET /api/rpc/cookbook`).

## Workflow index

Each subsection will grow **copy-paste examples** (ROADMAP Phase 12). Today: method names + one-line help in the UI.

### Chain & sync

| Workflow | Methods | DogeGo notes |
|----------|---------|--------------|
| Tip / height | `getblockchaininfo`, `getblockcount`, `getbestblockhash`, `getinfo`, `getmininginfo` | `blocks` = contiguous bodies; `headers` may run ahead during IBD; deprecated `getinfo` includes sync fields |
| Sync / IBD | `getblockchaininfo` (`dogego_*` fields) | `verificationprogress` = body coverage; `dogego_tx_verification_progress` = Core tx curve (mainnet); `initialblockdownload` uses bodies, `nMinimumChainWork`, and `-maxtipage`; `dogego_sync_eta`, `dogego_sync_status`, `dogego_blocks_behind_headers`, `dogego_raw_sync` |
| Assume-valid | `getblockchaininfo` | `dogego_assumevalid`, `dogego_assumevalid_height`; CLI/config `-assumevalid` / `"assumevalid": "0"` to verify all scripts |
| Block data | `getblock`, `getblockheader`, `getrawtransaction` | Needs `rawblocks/`; tx index for confirmed txs |
| Filters | `getblockfilter`, `getblockfilterheader`, `scanblocks`, `reindexblockfilters` | BIP158 **basic**; `scanblocks` uses persisted filters |
| UTXO set | `gettxout`, `gettxoutsetinfo`, `scantxoutset`, `dumptxoutset`, `loadtxoutset` | Native in-memory UTXO cache + scans over `rawblocks/` (no Core `chainstate/`) |
| Control | `invalidateblock`, `reconsiderblock`, `verifychain`, `waitforblock*` | |

### Mempool & relay

| Workflow | Methods | DogeGo notes |
|----------|---------|--------------|
| Submit tx | `sendrawtransaction`, `testmempoolaccept` | Witness txs rejected at admission |
| Package | `submitpackage` | Parent→child topological package; CPFP package feerate; `effective-includes` / `replaced-transactions` |
| Policy | `getmempoolinfo`, `getrawmempool`, `setmempoolpaused` | `dogego_*` policy fields on info |
| Persist | `savemempool`, `loadmempool`, `importmempool` | JSON `dogego_mempool.json` only |

### Mining & fees

| Workflow | Methods | DogeGo notes |
|----------|---------|--------------|
| Template | `getblocktemplate`, `submitblock`, `createauxblock`, `getauxblock` | AuxPoW era on mainnet |
| Estimates | `estimatesmartfee`, `estimatefee`, `estimaterawfee` | Heuristic / confirm-stats - not full Core estimator |
| Local mine | `generate`, `generatetoaddress` | Testnet / regtest-style local blocks |

### Raw transactions & PSBT

| Workflow | Methods | DogeGo notes |
|----------|---------|--------------|
| PQ commitments | `dogego_verifypqcommitment`, `createrawtransaction` (`pqcommit`) | Off-chain format verify; recognition in `decodescript` |
| Build | `createrawtransaction`, `combinerawtransaction` | Legacy wire; no witness |
| Fund / sign | `fundrawtransaction`, `signrawtransaction`, `signrawtransactionwithkey`, `signrawtransactionwithwallet` | Wallet + UTXO cache |
| PSBT | `createpsbt`, `converttopsbt`, `decodepsbt`, `analyzepsbt`, `combinepsbt`, `joinpsbts`, `utxoupdatepsbt`, `finalizepsbt` | No BIP32 deriv paths in PSBT |
| Wallet PSBT | `walletcreatefundedpsbt`, `walletprocesspsbt`, `descriptorprocesspsbt`, `psbtbumpfee` | Built-in wallet only |

### Wallet (built-in)

See [WALLET.md](WALLET.md).

### Network

| Workflow | Methods |
|----------|---------|
| Peers | `getpeerinfo`, `getconnectioncount`, `addnode`, `disconnectnode`, `setban`, `clearbanned`, `listbanned` |
| Addrman | `getnodeaddresses`, `getaddrmaninfo` |
| Info | `getnetworkinfo`, `getnettotals`, `getaddednodeinfo` |

**Integrator APIs (dashboard, loopback):** `GET /api/rpc/cookbook`, `GET /api/openrpc.json`, `GET /api/rpc/reference.html`, `GET /api/integration/guides`.

**DogeGo extensions:** `dogego_recoverheaders`, `dogego_verifypqcommitment` (off-chain Phase-1 PQ OP_RETURN format check).

## `dogego_*` fields

Many responses include extra keys explaining DogeGo behavior (storage model, sync state, policy). Do not rely on them in cross-node integrations.

## Contributing examples

Add a subsection under the right workflow with:

```bash
curl -s --user 'USER:PASS' -H 'content-type: application/json' \
  -d '{"jsonrpc":"1.0","id":1,"method":"METHOD","params":[...]}' \
  http://127.0.0.1:22555/
```

Then mirror a short summary in `ui/docs_index.go` for the **Docs** tab.
