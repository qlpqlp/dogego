# Submitting ZK proofs (dogego.zkl2)

This guide explains how to submit a **tx-anchored ZK proof** on the optional DogeGo ZK L2 extension and how it **syncs decentralized** with other DogeGo nodes.

**No Dogecoin consensus fork.** Proofs are verified off-L1 inside the extension (`OP_CHECKZKP` analogue). Dogecoin Core and nodes without the extension ignore overlay traffic.

## 1. Enable the extension

```bash
dogego-cli dogego_enableextension dogego.zkl2
```

Or: **Settings → Extensions → Enable** on `DogeGo ZK Layer 2`.

When enabled, your node:

1. Negotiates **`exthello` / `extack`** after the normal Dogecoin `verack`
2. Advertises overlay protocol **`zkproof-v1`**
3. Syncs proofs with peers that also negotiated `zkproof-v1`

## 2. Generate a proof from text or a file

The extension can **generate** proofs locally (no wallet keys). Two kinds:

| `proof_kind` | Meaning |
|--------------|---------|
| `commitment` (default) | SHA256 hash commitment to your text/file bytes. Verifiable overlay proof bound to a confirmed tx. **Not** a Groth16 SNARK. |
| `groth16` | Assemble a Groth16 proof object. Supply `groth16_proof_hex` from an external prover (snarkjs/circom), or `demo_groth16: true` for a pairing smoke test only. |

```bash
dogego-cli dogego_ext_dogego_zkl2_generateproof '{
  "payload": "much wow",
  "payload_encoding": "text",
  "proof_kind": "commitment"
}'
```

For a file, base64-encode bytes and use `"payload_encoding": "base64"`.

The response includes:

- `proof` full proof object (add tx anchor fields before submit)
- `payload_sha256` hash of your input
- `zkdg_script_hex` optional `ZKDG` OP_RETURN script (secondary L1 marker per [#3869](https://github.com/dogecoin/dogecoin/discussions/3869))

### Optional submit in one call

After your anchor transaction confirms:

```bash
dogego-cli dogego_ext_dogego_zkl2_generateproof '{
  "payload": "much wow",
  "payload_encoding": "text",
  "proof_kind": "commitment",
  "transaction_id": "<txid>",
  "block_hash": "<block_hash>",
  "block_height": 1234567,
  "submit": true
}'
```

## 3. Anchor a proof to a confirmed transaction

Every proof must reference a **confirmed** Dogecoin transaction:

| Field | Meaning |
|-------|---------|
| `transaction_id` | Tx that carries (or references) your proof payload |
| `block_hash` | Block containing that tx |
| `block_height` | Confirmation height |
| `proof_data` | Groth16 blob (hex) |
| `public_inputs` | Public inputs (required for sighash-style binding) |

The extension computes:

`proof_hash = SHA256(canonical proof fields)`  
`commitment = SHA256(block_hash || transaction_id || proof_hash)`

### Typical workflow

1. **Broadcast your Dogecoin transaction** (with proof data in witness/OP_RETURN as your app defines).
2. Wait for confirmation (1+ blocks).
3. **Submit the proof object** via RPC:

```bash
dogego-cli dogego_ext_dogego_zkl2_submitproof '{
  "transaction_id": "<txid>",
  "block_hash": "<block_hash>",
  "block_height": 1234567,
  "proof_data": "<hex>",
  "proof_type": 1,
  "public_inputs": ["<hex>", "..."]
}'
```

4. On success the node:
   - Validates structure + chain binding + `VerifyCheckZKP` (extension-only)
   - Stores proof in local Pebble DB
   - Updates overlay **ProofRoot** for that block
   - Announces **`zkinv`** to all `zkproof-v1` peers

### Verify without storing

```bash
dogego-cli dogego_ext_dogego_zkl2_verifyproof '<proof json>'
dogego-cli dogego_ext_dogego_zkl2_checkzkp '<proof json>'
```

## 4. How decentralized sync works

```
Your node                    Peer (also has zkl2 enabled)
   |  exthello / extack (zkproof-v1)  |
   |  getzkheaders  →  zkheaders      |  compare proof counts
   |  getzkblockproofs / getzkproof   |
   |  ←  zkproof                      |  verify + store
   |  zkinv  →  (relay to other peers)|
```

| P2P command | Purpose |
|-------------|---------|
| `exthello` / `extack` | Capability handshake after `verack` |
| `zkinv` | Announce new `proof_hash` values |
| `getzkproof` | Request proofs by hash |
| `zkproof` | Deliver one or more proof JSON objects |
| `getzkheaders` | Request height → proof-count summary |
| `zkheaders` | Response with heights and counts |
| `getzkblockproofs` | Request all proofs for a block hash |

Only peers that negotiated **`zkproof-v1`** receive relayed `zkinv`. This is a **mesh overlay**, not a central server.

## 5. Query local state

```bash
dogego-cli dogego_ext_dogego_zkl2_info
dogego-cli dogego_ext_dogego_zkl2_getproof "<proof_hash>"
dogego-cli dogego_ext_dogego_zkl2_listproofs "<block_hash>"
dogego-cli dogego_ext_dogego_zkl2_proofroot "<block_hash>"
dogego-cli dogego_ext_dogego_zkl2_listanchors
```

## 6. Optional ZKDG OP_RETURN anchor

Secondary human-visible L1 marker (`ZKDG` tag + 32-byte anchor hash). Not required for proof binding.

```bash
# Unlock wallet first (operator RPC, not callable from extensions)
dogego-cli walletpassphrase "<passphrase>" 600

# Option A: one-shot sign via extension wallet_rpc
dogego-cli dogego_ext_dogego_zkl2_signanchor '{"l2_height":1,"parent_hash":"...","state_root":"...","proof_digest":"...","signer_address":"YourAddress"}'

# Option B: prepare digest, then sign yourself
dogego-cli dogego_ext_dogego_zkl2_prepareanchor '<l2 header json>'
dogego-cli signmessage "YourAddress" '<sign_message from prepareanchor>'
```

## 7. Proof wire format (ZKPG v1)

Recommended `proof_data` hex decodes to:

```
ZKPG | proof_len u32 | pi_count u32 | proof_bytes | public_inputs (pi_count × 32 bytes)
```

`public_inputs` in the JSON object must match the embedded 32-byte values. Legacy test blobs (≥32 bytes + JSON public inputs) still verify for development.

**Groth16 pairing (optional):** place a snarkjs-style compressed verifying key at:

`<datadir>/<network>/extensions/dogego.zkl2/data/vk/default.vk`

When present, ZKPG proofs with `proof_len = 192` (compressed G1+G2+G1) or `proof_len = 384` (#3869 DIP affine layout) are fully verified via BLS12-381 pairings against the loaded VK.

For script-faithful #3869 mode-0 flows without a datadir VK, pass inline key material on `verifyproof` / `checkzkp`:

| Field | Meaning |
|-------|---------|
| `verifying_key` | Flat snarkjs `.vk` hex |
| `verifying_key_chunks` | Six 80-byte hex chunks (`verifier data 0..5` stack pushes) |

These fields are verify-time only and are **not** included in `proof_hash`.

## 7. Security notes

- Extensions **cannot** access private keys directly
- With `wallet_rpc`, only allowlisted wallet JSON-RPC methods are available; unlock with `walletpassphrase` before sign/send
- Invalid proofs are rejected and never relayed
- `proof_data` max 256 KiB per proof
- Install only VK files you trust; pairing verify binds proofs to that circuit

## See also

- [ZK protocol reference](PROTOCOL.md)
- [Extensions overview](../EXTENSIONS.md)
