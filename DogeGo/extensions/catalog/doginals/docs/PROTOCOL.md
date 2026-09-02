# doginals-v1 protocol

Overlay for DogeGo peers that enable `dogego.doginals` (v0.4.0+). Negotiated via host `exthello` / `extack` (same machinery as `zkproof-v1`).

## Goals

- Share **off-L1** asset metadata (NFT / token / image / collection) among DogeGo nodes  
- Keep **L1 indexing** local (each node observes its own chain)  
- Expose **Doginals wallet** read APIs for wallets  
- Stay **observe-only** on Dogecoin consensus: no new opcodes, no soft/hard fork  

## Commands

| Command | Payload | Direction |
|---------|---------|-----------|
| `dinv` | Length-prefixed UTF-8 asset ids (`uint16` BE length + bytes, repeated) | Announce inventory |
| `getdasset` | UTF-8 asset id | Request one asset |
| `dasset` | JSON `Asset` object | Deliver asset |

On peer connect (and about every 60s), a node may send `dinv` with up to ~200 local ids. Receivers request missing ids with `getdasset`; owners reply with `dasset`. Creating an asset (`putasset` / `mintl2`) also broadcasts `dinv`.

## L1 inscription record

Stored when an OP_RETURN (or compatible data carrier) is seen:

```json
{
  "id": "<txid>i<vout>@<height>",
  "height": 123,
  "txid": "…",
  "vout": 0,
  "kind": "drc20|doginal|data",
  "tick": "DOGE",
  "op": "mint",
  "amount": "1000",
  "address": "D…",
  "recipient": "",
  "content_type": "application/json",
  "text_preview": "…",
  "payload_hex": "…"
}
```

### DRC-20 detection

JSON payload with `"p":"drc-20"` (or `drc20`) and a non-empty `tick`. Ops: `deploy` / `mint` / `transfer` (lowercased).

### Doginal-like detection

Payload text contains `doginal`, starts with `ord`, or mentions `text/plain` / `image/` → `doginal`. Other OP_RETURN data → `data`.

## Address ledger (`bal/`, `ah/`)

When a DRC-20 event has an `address`, the indexer updates:

- `bal/<addr>/<TICK>` → `{ tick, balance, transferable_balance, transfers_count }`  
- `ah/<addr>/<id>` → history row (`Mint` / `Transfer` / …)

`CreditL2Balance` (RPC `mintl2`) credits balances without an L1 tx (experimental L2).

## L2 asset record

```json
{
  "id": "hex16…",
  "kind": "nft|token|image|collection",
  "name": "Much Wow #1",
  "description": "",
  "content_type": "image/png",
  "uri": "ipfs://…",
  "content_b64": "",
  "l1_inscription_id": "",
  "created_unix": 0,
  "updated_unix": 0,
  "creator_note": ""
}
```

If `id` is omitted, it is derived as `SHA256(kind|name|uri|content_b64|l1_inscription_id)[:16]` hex.

## Token index (`tk/`)

Each DRC-20 tick accumulates a `TokenSummary` (deploy max/lim, mint/transfer counts, last height).

## HTTP API (extension-owned)

Host mounts a **generic** gateway: `/api/ext/{extension.id}/…` → RPC `httphandle`.
Doginals implements wallet read paths under `/api/ext/dogego.doginals/v1/*`. Core has **no** doginals-specific HTTP handlers. See [USER_GUIDE.md](USER_GUIDE.md).

## Mint paths

| Path | Mechanism |
|------|-----------|
| L1 `inscribe` | OP_RETURN ≤80 B → `createrawtransaction` → fund → sign → optional `sendrawtransaction` |
| L2 `mintl2` | Local ledger credit + optional L2 asset + `dinv` broadcast |

## Storage

Pebble DB `doginals.db` under the extension data directory:

| Key prefix | Value |
|------------|-------|
| `i/` | Inscription JSON |
| `ih/` | Height → id index |
| `x/` | Txid → id index |
| `t/` | Tick → id (DRC-20) |
| `tk/` | Token summary |
| `bal/` | Address balances |
| `ah/` | Address history |
| `a/` | Asset JSON |
| `m/` | Meta (`index_height`, `config`, …) |

## Non-goals / follow-ups

- Every exotic Ordinals tag (parent, metadata pointers, …) — body + content-type are indexed  
- WebSocket event subscriptions  
- Cryptographically anchored L2 consensus (L2 mint remains experimental gossip)
