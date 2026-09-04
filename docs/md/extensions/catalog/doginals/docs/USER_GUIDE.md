# Doginals / DRC-20 L2 - user guide

Extension id: **`dogego.doginals`** (v0.8.0). Overlay protocol: **`doginals-v1`**.

Works on **mainnet and testnet**. Experimental - not a Dogecoin consensus change.

## What it does

| Layer | Behavior |
|-------|----------|
| **L1 index** | Indexes **classic P2SH Doginals** ([apezord](https://github.com/apezord/doginals) / [booktoshi](https://github.com/booktoshi/doginals)), **Ord envelopes**, and **OP_RETURN** that already exist on Dogecoin. Classifies **token / image / text / file**, stores media, builds balances. |
| **L2 mint (only mint path)** | When this extension is enabled, **all minting is L2**. Tokens, images, files, and Ordinals (`kind=ordinal`, official `ord` envelope). Wallet `signmessage`. Gossiped to peers. **No L1 mint** of Doginals, Ordinals, P2SH, or OP_RETURN. |
| **Wallet API** | REST under `/api/ext/dogego.doginals/v1/*`. |
| **UI** | Wizard: Setup → Sync → Create (L2 token / L2 image-file / L2 Ordinals) → Wallet API → Browse. |

## Mental model

> **L1** = observe the Dogecoin chain. This node indexes P2SH / OP_RETURN / Ord envelopes that others already wrote on-chain. It does not write those inscriptions.  
> **L2** = the only mint this extension offers. Wallet-signed records sync among Doginals-enabled DogeGo nodes. Verifiable by signature + content hash. Not L1 consensus.

## How sync works (decentralized, permissionless)

1. You enable `dogego.doginals` on your DogeGo node. No signup, no registrar, no trusted server.
2. **L1:** as blocks connect (and via `indexrange` backfill), this node scans txs and stores inscriptions locally. Every node does its own index from the same chain.
3. **L2:** peers that also enabled the extension negotiate `doginals-v1` over P2P (`exthello` / `extack`), same style as other DogeGo overlays.
4. On connect and about every 60 seconds, a node may announce inventory: `dinv` (assets) and `dminv` (signed mints).
5. Neighbors request missing ids (`getdasset` / `getdmint`) and receive `dasset` / `dmint`.
6. A received mint is **dropped** unless `signmessage` verification succeeds for the P2PKH address, the nonce is unused, and optional `content_hash` matches the body.
7. Anyone can ignore L2. Only nodes that run this extension participate. Invalid or unsigned gossip never becomes an L1 coin.

That is permissionless among operators: run DogeGo, enable the extension, connect to peers. There is no central indexer you must trust for L2 mints; you verify signatures yourself.

## Install

```powershell
.\build-universal.ps1
```

Settings → Extensions → Install zip → `dist/doginals-universal.zip` → **Enable**.

Enable **wallet RPC** in Step 1 and unlock the dashboard wallet for one-click L2 mint signing.

## Create / mint (L2 only)

### L2 token

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

If wallet RPC is unlocked, the extension signs and commits. Otherwise the response includes `sign_message` - sign with `signmessage`, then:

```http
POST /api/ext/dogego.doginals/v1/mint/commit
{ "record": {…}, "signature": "<base64>" }
```

### L2 image or file

Wizard **Mint image or file on L2** (Choose file), or POST `/mint` with `kind=image|file` and `content_b64`. Max **4 MiB**.

### L2 Ordinals

Wizard **Mint Ordinals on L2**, or:

```http
POST /api/ext/dogego.doginals/v1/mint
{
  "kind": "ordinal",
  "op": "inscribe",
  "address": "D…",
  "name": "Ordinal #1",
  "content_type": "image/png",
  "content_b64": "<base64 bytes>"
}
```

The response includes `envelope_hex`: official `OP_FALSE OP_IF "ord" OP_1 <type> OP_0 <body> OP_ENDIF`. The envelope is stored on L2 with the signed record. This extension does **not** put it in a Dogecoin witness.

`mintp2sh`, `inscribe`, and `destination=p2sh|opreturn` are **rejected** (mint is L2 only).

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
| POST | `/mint` | L2 mint (token/image/file/ordinal) |
| POST | `/mint/prepare` | Unsigned record + `sign_message` |
| POST | `/mint/commit` | Commit signed mint |
| POST | `/mint/l2` | Alias of `/mint` |

`media_kind`: `token` \| `image` \| `text` \| `json` \| `file`.

## RPC

Prefix: `dogego_ext_dogego_doginals_<method>`

| Method | Purpose |
|--------|---------|
| `mint` / `mintl2` | L2 mint (auto-sign if wallet unlocked) |
| `mintprepare` / `mintcommit` | Two-step external sign flow |
| `listmints` / `getmint` / `getmintcontent` | L2 mint gallery |
| `listinscriptions` / `getinscription` / `getcontent` | L1 index + media |
| `indexrange` | Backfill L1 heights |
| `getaddress` / `listtokens` / … | Wallet reads |
| `inscribe` / `mintp2sh` | Disabled (returns L2-only error) |

## Wallet integration

1. Point the wallet at the node: `/api/ext/dogego.doginals/v1`
2. Create new tokens/images/files/ordinals with `POST /mint` (L2)
3. Read historical on-chain Doginals via `/inscription/…`
4. Verify L2 mints with Dogecoin `verifymessage` against canonical JSON

## Limits / honesty

- L2 is **not** Dogecoin consensus; only **valid signatures** are accepted
- Deep reorgs may need reindex for L1
- Requires **txindex** for best address attribution on L1
- P2SH multi-part chains are assembled as blocks are indexed
