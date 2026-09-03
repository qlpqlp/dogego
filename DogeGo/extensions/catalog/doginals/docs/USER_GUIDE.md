# Doginals / DRC-20 L2 - user guide

Extension id: **`dogego.doginals`** (v0.7.0). Overlay protocol: **`doginals-v1`**.

Works on **mainnet and testnet**. Experimental — not a Dogecoin consensus change.

## What it does

| Layer | Behavior |
|-------|----------|
| **L1 index** | Indexes **classic P2SH Doginals** ([apezord](https://github.com/apezord/doginals) / [booktoshi](https://github.com/booktoshi/doginals)), **Ord envelopes**, and **OP_RETURN**. Classifies **token / image / text / file**, stores media, builds balances + transferable UTXOs. Auto catch-up + soft reorg. |
| **L2 mint (default)** | When this extension is **enabled**, minting goes to **signed L2** (not Dogecoin). Supports **DRC-20-style tokens**, **images**, and **files**. Each mint is signed with `signmessage` and gossiped to peers. |
| **L1 mint (optional)** | Advanced: OP_RETURN DRC-20 via `inscribe`. **No P2SH mint builder** — P2SH is index-only. |
| **Wallet API** | REST under `/api/ext/dogego.doginals/v1/*`. |
| **UI** | Wizard: Setup → Sync → Create (L2 token / L2 image-file / optional L1) → Wallet API → Browse. |

## Mental model

> **L1** = anyone can verify from the Dogecoin chain (index P2SH / OP_RETURN / envelopes).  
> **L2** = wallet-signed mint records synced among Doginals-enabled DogeGo nodes. Verifiable by signature + optional content hash — not L1 consensus.

## Install

```powershell
.\build-universal.ps1
```

Settings → Extensions → Install zip → `dist/doginals-universal.zip` → **Enable**.

Enable **wallet RPC** in Step 1 and unlock the dashboard wallet for one-click L2 mint signing.

## Create / mint

### L2 token (default)

Wizard **Mint DRC-20 on L2**, or:

```http
POST /api/ext/dogego.doginals/v1/mint
Content-Type: application/json

{
  "kind": "token",
  "op": "mint",
  "address": "D…",
  "tick": "WOOF",
  "amount": "1000"
}
```

If wallet RPC is unlocked, the extension signs and commits. Otherwise the response includes `sign_message` — sign with `signmessage`, then:

```http
POST /api/ext/dogego.doginals/v1/mint/commit
{ "record": {…}, "signature": "<base64>" }
```

### L2 image or file

Wizard **Mint image or file on L2** (file picker), or:

```http
POST /api/ext/dogego.doginals/v1/mint
{
  "kind": "image",
  "op": "inscribe",
  "address": "D…",
  "name": "Much Wow #1",
  "content_type": "image/png",
  "content_b64": "<base64 bytes>"
}
```

Max body **4 MiB**. Peers verify `content_hash` + signature before accepting.

### Optional L1 OP_RETURN

Use wizard **Advanced: DRC-20 on L1** or `POST …/inscribe`. Writes to Dogecoin. Prefer L2 when using DogeGo.

## HTTP API

Base: `https://<node>:2013/api/ext/dogego.doginals/v1` (or `http://` with `-notls`).

| Method | Path | Notes |
|--------|------|-------|
| GET | `/status` | Index height, tip, lag |
| GET | `/tokens` | Token summaries |
| GET | `/address/{addr}` | Balances |
| GET | `/address/{addr}/history?tick=` | History |
| GET | `/txid/{txid}` | L1 events for tx |
| GET | `/inscription/{id}` | L1 inscription metadata |
| GET | `/inscription/{id}/content` | L1 media (`content_b64`, `data_url`) |
| GET | `/mints` | Recent signed L2 mints |
| GET | `/mint/{id}` | One L2 mint |
| GET | `/mint/{id}/content` | L2 media |
| POST | `/mint` | L2 mint (token/image/file) — unlock |
| POST | `/mint/prepare` | Unsigned record + `sign_message` |
| POST | `/mint/commit` | Commit signed mint |
| POST | `/mint/l2` | Alias of `/mint` |
| POST | `/inscribe` | Optional L1 OP_RETURN |

`media_kind`: `token` \| `image` \| `text` \| `json` \| `file`.

## RPC

Prefix: `dogego_ext_dogego_doginals_<method>`

| Method | Purpose |
|--------|---------|
| `mint` / `mintl2` | L2 mint (auto-sign if wallet unlocked) |
| `mintprepare` / `mintcommit` | Two-step external sign flow |
| `listmints` / `getmint` / `getmintcontent` | L2 mint gallery |
| `listinscriptions` / `getinscription` / `getcontent` | L1 index + media |
| `inscribe` | Optional L1 OP_RETURN |
| `indexrange` | Backfill L1 heights |
| `getaddress` / `listtokens` / … | Wallet reads |

## Wallet integration

1. Point the wallet at the node: `/api/ext/dogego.doginals/v1`  
2. Prefer `POST /mint` for new tokens/images/files when the extension is enabled  
3. Keep reading L1 inscriptions via `/inscription/…` for classic Doginals already on-chain  
4. Verify L2 mints by checking `signature` with Dogecoin `verifymessage` against `sign_message` / canonical JSON  

## Limits / honesty

- L2 is **not** Dogecoin consensus; anyone can ignore unsigned gossip — only **valid signatures** are accepted  
- Deep reorgs may need reindex for L1  
- Requires **txindex** for best address attribution on L1  
- P2SH multi-part chains are assembled as blocks are indexed  
