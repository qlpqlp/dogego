# BBPoW user guide (research)

## What this is

`dogego.bbpow` is a **testnet-only experimental** DogeGo extension for **Bitcoin-Backed Proof-of-Work (BBPoW)** / **CAuxPoW**.

It explores whether Dogecoin could one day count **Bitcoin SHA-256** work (commitment in a Bitcoin coinbase) alongside today’s **Scrypt AuxPoW** (Litecoin).

**Experimental:** off-L1 only. Does not change what blocks your node accepts.

## ASICs (read this first)

- Bitcoin miners use **SHA-256 ASICs**. Those chips **cannot** mine Scrypt AuxPoW.  
- Under a future **hard fork** design, those same SHA-256 ASICs could secure a Dogecoin “Bitcoin lane” by embedding a commitment while mining Bitcoin - **no second ASIC** for that lane.  
- Litecoin/Dogecoin merge miners still need **Scrypt ASICs** for classic AuxPoW.  
- One ASIC ≠ both algorithms. “Both” means two hardware classes, or two lanes after a hard fork.

## Soft fork vs hard fork

| Want | Possible? |
|------|-----------|
| Bitcoin work **instead of** Scrypt, old nodes still follow | **No** → hard fork |
| Same AuxPoW **wire** shape with Bitcoin as parent | Wire yes; rules must change → still hard fork for OR |
| Soft fork that only **adds** extra commitments while Scrypt AuxPoW stays mandatory | Yes, but Bitcoin-only miners still cannot produce blocks alone |

Full write-up: [PROTOCOL.md](PROTOCOL.md), [HARD_FORK.md](HARD_FORK.md).

## What this is not

| Claim | Reality |
|-------|---------|
| "AuxPoW for Bitcoin miners" | **Misleading.** Classic AuxPoW needs matching PoW algos. |
| Soft fork to admit Bitcoin-only work | **No** (see above). |
| Mainnet consensus change | **No.** Extension verifies proofs off L1. |
| Built-in Bitcoin node | **No.** You pass headers / coinbase / merkle to RPC. |

## Enable

1. Run DogeGo on **reboot testnet**.  
2. Build or obtain `bbpow.zip` (`build.ps1` / `build.sh`).  
3. Extensions → Install zip → Enable `dogego.bbpow`.  
4. Panel / RPC:

```text
dogego_ext_dogego_bbpow_info
dogego_ext_dogego_bbpow_compare
dogego_ext_dogego_bbpow_dualmodel
```

## Useful RPCs

| Method | Purpose |
|--------|---------|
| `buildcommitment` | `[doge_block_hash_hex]` → commitment for a Bitcoin coinbase |
| `checkheader` | Verify SHA-256 PoW on an 80-byte Bitcoin header |
| `verifyproof` | Full research proof object |
| `dualmodel` | Scrypt vs SHA-256 lane sketch |
| `compare` | AuxPoW vs BBPoW, soft/hard fork, ASICs |

## Safety

- Refuses **mainnet** enable.  
- No wallet keys.  
- Does not accept/reject L1 blocks.  
- Research telemetry only.
