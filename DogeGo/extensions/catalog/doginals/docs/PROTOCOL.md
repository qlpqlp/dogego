# doginals-v1 protocol

Overlay for DogeGo peers that enable `dogego.doginals` (v0.7.0+). Negotiated via host `exthello` / `extack` (same machinery as `zkproof-v1`).

## Goals

- Share **signed off-L1** mints (token / image / file) among DogeGo nodes  
- Keep **L1 indexing** local (P2SH Doginals, Ord envelopes, OP_RETURN)  
- Expose **Doginals wallet** read APIs + **content** for wallets  
- Stay **observe-only** on Dogecoin consensus: no new opcodes, no soft/hard fork  
- **Mint on L2 when the extension is enabled**; do **not** build L1 P2SH mints (index only)

## Commands

| Command | Payload | Direction |
|---------|---------|-----------|
| `dinv` | Length-prefixed UTF-8 asset ids | Announce L2 asset inventory |
| `getdasset` | UTF-8 asset id | Request one asset |
| `dasset` | JSON `Asset` | Deliver asset |
| `dminv` | Length-prefixed mint ids | Announce signed L2 mints |
| `getdmint` | UTF-8 mint id | Request one mint |
| `dmint` | JSON `{ "record": L2MintRecord, "content_b64"?: "…" }` | Deliver mint (+ optional body) |

On peer connect (and ~every 60s), a node may send `dinv` / `dminv`. Receivers request missing ids. Creating a mint broadcasts `dminv`.

Receivers **must** verify `record.signature` with Dogecoin `signmessage` rules before `PutL2Mint`. Invalid or replayed nonces are dropped.

## L1 indexing (P2SH / envelope / OP_RETURN)

### Classic P2SH Doginals ([apezord](https://github.com/apezord/doginals))

Inscription pushdatas in **scriptSig** (redeem path):

```
"ord" | pieces | content-type | (n, data)* …
```

Parts may span multiple txs; separators count down to `0`. Indexer assembles pending state keyed by the next P2SH outpoint. **Minting P2SH on L1 is not implemented** — historical and future P2SH reveals are indexed only.

### Witness envelopes

`OP_FALSE OP_IF … "ord" … OP_ENDIF` in witness stack.

### OP_RETURN

DRC-20 JSON and data carriers (≤80 B typical).

## Signed L2 mint record (`doginals-l2` v1)

```json
{
  "id": "hex16…",
  "p": "doginals-l2",
  "v": 1,
  "op": "mint|deploy|transfer|inscribe",
  "kind": "token|image|file|nft",
  "tick": "WOOF",
  "amt": "1000",
  "address": "D…",
  "to": "D…",
  "name": "…",
  "content_type": "image/png",
  "content_hash": "sha256 hex",
  "nonce": "hex",
  "created_unix": 1730000000,
  "network": "mainnet|testnet",
  "signature": "base64 compact signmessage",
  "media_kind": "token|image|text|json|file",
  "size": 1234,
  "has_content": true
}
```

### Signing

1. Build record with `nonce` + `created_unix` + `content_hash` (body stored separately).  
2. `sign_message` = canonical JSON of the record **without** `signature` / `recorded_unix`, and **without** `content_b64` when `content_hash` is set.  
3. Wallet: `signmessage(address, sign_message)` → base64 compact ECDSA.  
4. Peers: recover pubkey, match P2PKH `address`, check nonce uniqueness, optional body vs `content_hash`.

### Apply rules

- `kind=token` + `op=mint|deploy` + `amt` → credit L2 balance for `to` (or `address`)  
- `kind=image|file|nft` → store body; expose via `/mint/{id}/content`  
- Duplicate `id` or reused `nonce` → reject  

## L1 inscription record

See USER_GUIDE. Fields include `source` (`p2sh` \| `envelope` \| `opreturn`), `media_kind`, `has_content`.

## Address ledger

L1 DRC-20 events and verified L2 token mints update `bal/` + `ah/`.

## HTTP surface

All under `/api/ext/dogego.doginals/v1/*` via extension `httphandle`. Core only proxies `/api/ext/{id}/…`.

## Honesty

L2 mints are **not** Dogecoin consensus. Anyone running Doginals can verify signatures and sync; they cannot force the wider chain to accept L2 balances. Optional future work: Merkle roots anchored in OP_RETURN for stronger timestamps.
