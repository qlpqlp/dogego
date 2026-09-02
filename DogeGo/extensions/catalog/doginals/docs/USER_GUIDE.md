# Doginals / DRC-20 L2 - user guide

Extension id: **`dogego.doginals`** (v0.5.0). Overlay protocol: **`doginals-v1`**.

Works on **mainnet and testnet**. Experimental — not a Dogecoin consensus change.

## What it does

| Layer | Behavior |
|-------|----------|
| **L1 index** | Indexes **Ord/Doginals envelopes** (witness `OP_FALSE OP_IF … ord …`) and **OP_RETURN** carriers. Classifies DRC-20 JSON, doginals, and data. Builds token summaries + **per-address balances** + **transferable UTXOs**. Auto catch-up + soft reorg. |
| **Wallet API** | Extension-owned REST via `/api/ext/dogego.doginals/v1/*` for wallets/sites. |
| **DRC-20 mint (L1)** | Wizard builds deploy/mint/transfer; optional broadcast via wallet RPC. |
| **L2 mint (off-chain)** | `mintl2` / `POST …/mint/l2` without writing Dogecoin; P2P asset sync. |
| **UI** | Wizard: Setup → Sync → Create → Wallet API (+ Browse). |

Wallet apps (e.g. MyDoge-class wallets) can point at a DogeGo node instead of a centralized API.

## Install

```powershell
.\build.ps1
```

Settings → Extensions → Install zip → `dist/doginals-universal.zip` (or `doginals.zip`) → Enable.

```bash
dogego-cli dogego_enableextension dogego.doginals
```

Or install from the GitHub catalog (`dogego.doginals` v0.4.0).

## Wizard flow

1. **Setup** — enable wallet RPC if you mint on L1; export backup  
2. **Sync** — watch L1 index lag; backfill heights (`indexrange`, max 5000/run); P2P overlay status  
3. **Create** — DRC-20 on Dogecoin (L1) or experimental L2 off-chain mint  
4. **Wallet API** — copy routes for wallet / site integrators  
5. **Browse** — tokens, inscriptions, L2 assets, address lookup  

Wallet rules:

- Manifest permission `wallet_rpc` + allowlisted methods only  
- Extension **cannot** call `walletpassphrase` or export keys  
- Unlock from the DogeGo UI before L1 broadcast  
- Toggle wallet use in **Step 1 — Setup**

## HTTP API (wallet reads)

Base URL: `https://<node>:2013/api/ext/dogego.doginals/v1` (or `http://` when `-notls`).

The **route logic lives in this extension** (`httphandle` RPC). DogeGo only provides a generic proxy at `/api/ext/{extension.id}/…` — there is no `/api/doginals` code in core.

| Method | Path | Notes |
|--------|------|-------|
| GET | `/status` | Index height, chain tip, lag |
| GET | `/tokens` | Token summaries |
| GET | `/address/{addr}` | Balances for address |
| GET | `/address/{addr}/history?tick=` | Event history |
| GET | `/txid/{txid}` | Events for txid |
| GET | `/` or `/v1` | API manifest (`apistatus`) |
| POST | `/mint/l2` | Off-L1 mint (unlock required) |
| POST | `/inscribe` | L1 DRC-20 (unlock required) |

Example:

```http
GET /api/ext/dogego.doginals/v1/address/DYOURADDRESS
GET /api/ext/dogego.doginals/v1/address/DYOURADDRESS/history?tick=WOOF
```

## RPC (after enable)

Methods: `dogego_ext_dogego_doginals_<method>`

| Method | Purpose |
|--------|---------|
| `info` | Status + wizard UI workspace |
| `listtokens` / `gettoken` / `listbytick` | DRC-20 token index |
| `getaddress` / `getaddresshistory` | Address balances / history |
| `geteventsbytxid` | Events for a txid |
| `previewinscription` / `inscribe` | Build / broadcast OP_RETURN DRC-20 |
| `mintl2` | Experimental off-L1 mint |
| `listinscriptions` / `getinscription` / `indexrange` | L1 index |
| `putasset` / `getasset` / `listassets` | L2 assets |
| `getconfig` / `setconfig` | Extension settings |
| `syncstatus` / `apistatus` / `httphandle` | Overlay + public API routes / HTTP gateway |

### Preview a mint

```bash
dogego-cli dogego_ext_dogego_doginals_previewinscription '{
  "op":"mint","tick":"woof","amt":"100"
}'
```

### Inscribe (wallet unlocked + Setup enabled)

```bash
dogego-cli dogego_ext_dogego_doginals_setconfig '{"wallet_rpc_enabled":true}'
dogego-cli dogego_ext_dogego_doginals_inscribe '{
  "op":"mint","tick":"woof","amt":"100","broadcast":true
}'
```

Payload must fit **80 bytes** OP_RETURN (standard DRC-20 JSON does).

## Limits (honest)

- Needs **txindex** for best sender address resolution (`LookupTxHex`)  
- Envelope parser covers standard ord/doginal tags (content-type + body); exotic tags ignored  
- L2 mint is **experimental** gossip among DogeGo peers, not L1 consensus  
- Soft reorg rolls back by height; deep reorgs may need a full reindex  

## See also

- [PROTOCOL.md](PROTOCOL.md) — wire format and storage  
- [README.md](README.md) — package overview  
- Host docs: `DogeGo/docs/EXTENSIONS.md`
