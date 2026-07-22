# Doginals / DRC-20 L2 - user guide

Extension id: **`dogego.doginals`**. Overlay protocol: **`doginals-v1`**.

Works on **mainnet and testnet**. Experimental - not a consensus change.

## What it does

| Layer | Behavior |
|-------|----------|
| **L1 index** | Watches blocks for OP_RETURN / data-carrier outputs. Classifies DRC-20 JSON (`"p":"drc-20"`), doginal-like payloads, and generic data. Builds a **token summary index** per ticker. |
| **DRC-20 mint** | Workspace wizard builds deploy / mint / transfer JSON and can **fund + sign + broadcast** an OP_RETURN tx via **authenticated wallet RPC** (Settings must enable it; unlock wallet in the dashboard). |
| **L2 assets** | Create NFT / token / image / collection records off-chain; sync via P2P among peers. |
| **UI** | Modern **workspace**: Home stays light; menu opens Tokens, Mint/Deploy wizard, Inscriptions, L2 Assets, Index, Settings. |

## Install

```powershell
.\build.ps1
```

Settings -> Extensions -> Install zip -> `dist/doginals.zip` -> Enable.

```bash
dogego-cli dogego_enableextension dogego.doginals
```

## Workspace flow

1. **Home** - status chips and counts only  
2. **Tokens** - list indexed DRC-20 tickers; open one with Get token  
3. **Mint / Deploy** - wizard: op -> params -> optional broadcast  
4. **Settings** - enable `wallet_rpc_enabled` before minting  

Wallet rules:

- Manifest permission `wallet_rpc` + allowlisted methods only (`createrawtransaction`, `fundrawtransaction`, `signrawtransactionwithwallet`, `sendrawtransaction`, …)
- Extension **cannot** call `walletpassphrase` or export keys
- Unlock from the DogeGo UI / Console (dashboard auth / cookie) before broadcast
- Toggle wallet use inside this extension's **Settings** tab

## RPC (after enable)

Methods: `dogego_ext_dogego_doginals_<method>`

| Method | Purpose |
|--------|---------|
| `info` | Status + UI workspace |
| `listtokens` / `gettoken` / `listbytick` | DRC-20 token index |
| `previewinscription` / `inscribe` | Build / broadcast OP_RETURN DRC-20 |
| `listinscriptions` / `getinscription` / `indexrange` | L1 index |
| `putasset` / `getasset` / `listassets` | L2 assets |
| `getconfig` / `setconfig` | Extension settings |
| `syncstatus` | Overlay hint |

### Preview a mint

```bash
dogego-cli dogego_ext_dogego_doginals_previewinscription '{
  "op":"mint","tick":"woof","amt":"100"
}'
```

### Inscribe (wallet unlocked + Settings enabled)

```bash
dogego-cli dogego_ext_dogego_doginals_setconfig '{"wallet_rpc_enabled":true}'
dogego-cli dogego_ext_dogego_doginals_inscribe '{
  "op":"mint","tick":"woof","amt":"100","broadcast":true
}'
```

Payload must fit **80 bytes** OP_RETURN (standard DRC-20 JSON does).

## See also

- [PROTOCOL.md](PROTOCOL.md) - wire format and kinds  
- Wallet bridge: `DogeGo/docs/EXTENSIONS.md`, `AUTHORING.md` (`wallet_rpc`)
