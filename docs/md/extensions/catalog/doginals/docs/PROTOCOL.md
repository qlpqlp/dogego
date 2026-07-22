# doginals-v1 protocol

Overlay for DogeGo peers that enable `dogego.doginals`. Negotiated like other extension protocols via host `exthello` / `extack` (same machinery as `zkproof-v1`).

## Goals

- Share **off-L1** asset metadata (NFT / token / image / collection) among DogeGo nodes  
- Keep **L1 indexing** local (each node observes its own chain)  
- Stay **observe-only** on consensus: no new opcodes, no soft/hard fork  

## Commands

| Command | Payload | Direction |
|---------|---------|-----------|
| `dinv` | Length-prefixed UTF-8 asset ids (`uint16` BE length + bytes, repeated) | Announce inventory |
| `getdasset` | UTF-8 asset id | Request one asset |
| `dasset` | JSON `Asset` object | Deliver asset |

On peer connect, a node may send `dinv` with up to ~200 local ids. Receivers request missing ids with `getdasset`; owners reply with `dasset`.

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
  "content_type": "application/json",
  "text_preview": "…",
  "payload_hex": "…"
}
```

### DRC-20 detection

JSON payload with `"p":"drc-20"` (or `drc20`) and a non-empty `tick`. Ops: `deploy` / `mint` / `transfer` (lowercased). This is a **heuristic indexer**, not a full DRC-20 balance engine.

### Doginal-like detection

Payload text contains `doginal`, starts with `ord`, or mentions `text/plain` / `image/` - classified as `doginal`. Other OP_RETURN data → `data`.

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

Each DRC-20 tick accumulates a `TokenSummary` (deploy max/lim, mint/transfer counts, last height). Not a full balance ledger.

## Mint path (wallet_rpc)

`inscribe` builds OP_RETURN hex (<=80 B), then `createrawtransaction` -> `fundrawtransaction` -> `signrawtransactionwithwallet` -> optional `sendrawtransaction`. Requires extension Settings `wallet_rpc_enabled` and an unlocked wallet via authenticated DogeGo UI/RPC.

## Storage

Pebble DB `doginals.db` under the extension data directory:

| Key prefix | Value |
|------------|-------|
| `i/` | Inscription JSON |
| `ih/` | Height → id index |
| `t/` | Tick → id (DRC-20) |
| `a/` | Asset JSON |
| `m/` | Meta (`index_height`, …) |

## Non-goals (v0.1)

- Full Ordinals/Doginals envelope parsing beyond OP_RETURN heuristics  
- Token balance / UTXO-bound DRC-20 ledger  
- Writing inscriptions to L1 from the extension  
- Guaranteed global uniqueness of L2 ids across adversarial peers (trust your peer set)
