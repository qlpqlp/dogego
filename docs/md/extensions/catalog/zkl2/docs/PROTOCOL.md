# zkproof-v1 overlay (extension `dogego.zkl2`)

**P2P protocol id:** `zkproof-v1`  
**Extension package id:** `dogego.zkl2`  
**Status:** v0.2 scaffold (optional; no Dogecoin consensus changes)

**Reference:** [Dogecoin #3869 OP_CHECKZKP discussion](https://github.com/dogecoin/dogecoin/discussions/3869)

## L1 vs L2: where OP_CHECKZKP runs

| Layer | OP_CHECKZKP / Groth16 verify |
|-------|------------------------------|
| **Dogecoin L1** | **Not available** - no soft fork, no opcode change |
| **DogeGo zkproof-v1 extension** | **Yes** - `VerifyCheckZKP()` is the #3869 analogue, off-chain only |

Dogecoin Core and non-extension DogeGo nodes **ignore** overlay traffic and treat L1 txs normally. Only peers that negotiate `zkproof-v1` after `verack` exchange proofs.

## Architecture (your spec)

```
Dogecoin P2P (unchanged)
  version → verack → inv / block / tx / headers / ...

DogeGo overlay (optional, after verack)
  exthello → extack
    zkproof-v1:
    zkinv / getzkproof / zkproof
    getzkheaders / zkheaders (height + proof counts)
    getzkblockproofs (all proofs for a block)
```

## Proof object

Each proof is anchored to an **immutable confirmed transaction**:

| Field | Role |
|-------|------|
| `proof_hash` | Deterministic SHA256(canonical fields) |
| `transaction_id` | Confirmed Dogecoin tx |
| `block_hash` | Block containing the tx |
| `block_height` | Confirmation height |
| `transaction_index` | Index in block (verified via txindex) |
| `proof_data` | Groth16 blob (hex) |
| `proof_type` | Mode `1` = Groth16 (OP_CHECKZKP mode 1 analogue) |
| `public_inputs` | Required (addresses #3869 sighash binding concern) |

**Commitment (overlay):**

`SHA256(block_hash || transaction_id || proof_hash)`

Because `block_hash` is fixed by Dogecoin consensus, the proof cannot be reassigned to another tx.

Optional `ZKDG` OP_RETURN outputs remain a **secondary** human-visible anchor; primary binding is tx + block hash.

## Validation pipeline

Every received proof (RPC or P2P):

1. Structural checks (version, size cap 256 KiB, hash consistency)
2. Chain checks (block exists at height, tx confirmed in block)
3. **`VerifyCheckZKP`** (extension-only; ZKPG wire v1 + optional Groth16 pairing when `data/vk/default.vk` is present)
4. Duplicate rejection
5. Store in Pebble + update **ProofRoot**

## ProofRoot (overlay only)

For each Dogecoin block, participating nodes:

1. Collect all stored proofs for `block_hash`
2. Sort by `transaction_id`
3. Merkle tree over `proof_hash` values
4. Store `proof_root` locally

**Never written** into Dogecoin block header or Merkle root.

## P2P sync flow

```
Peer A                    Peer B
  getzkproof [hashes]  →
                    ←  zkproof [json proofs]
  verify + store
  zkinv [hashes]     →  (relay to other zkproof-v1 peers)
```

Relay only to peers with negotiated `zkproof-v1`.

## RPC (extension enabled)

| RPC | Purpose |
|-----|---------|
| `dogego_ext_dogego_zkl2_submitproof` | Accept proof after full validation |
| `dogego_ext_dogego_zkl2_verifyproof` / `dogego_ext_dogego_zkl2_checkzkp` | Off-L1 OP_CHECKZKP verify |
| `dogego_ext_dogego_zkl2_getproof` | Load by `proof_hash` |
| `dogego_ext_dogego_zkl2_listproofs` | By `block_hash` |
| `dogego_ext_dogego_zkl2_proofroot` | Overlay ProofRoot for block |

## Security

- Extensions **cannot** access private keys directly
- Optional `wallet_rpc` permission: whitelisted wallet JSON-RPC only; sign/send needs `walletpassphrase` first
- Forbidden manifest permissions: `wallet`, `private_keys`, `sign_message`, `spend`
- Proof size limits + duplicate detection + chain binding
- Invalid proofs are never stored or relayed

## Comparison to your written spec

| Your requirement | DogeGo status |
|------------------|---------------|
| No consensus fork | Done |
| exthello / extack | Done |
| zkproof-v1 protocol id | Done |
| Proof object + tx/block anchor | Done |
| Pebble indexes | Done |
| zkinv / getzkproof / zkproof | Done |
| getzkheaders / zkheaders | Done |
| getzkblockproofs | Done |
| ProofRoot per block | Done |
| OP_CHECKZKP on L2 only | Done (compressed 192 B + DIP 384 B Groth16 pairing when VK loaded) |
| Full Groth16 pairing crypto | Done for snarkjs compressed proofs + `data/vk/*.vk` (192 B and DIP 384 B affine proofs); inline `verifying_key` / `verifying_key_chunks` on verify RPC |
| Wasm/subprocess third-party extensions | Supported (see [WASM_PROTOCOL.md](../WASM_PROTOCOL.md), [SUBPROCESS_PROTOCOL.md](../SUBPROCESS_PROTOCOL.md)) |
| Settings catalog UI | Done |

## Enable

```bash
dogego-cli dogego_enableextension dogego.zkl2
```

Extension auto-registers P2P protocol `zkproof-v1` when enabled.
