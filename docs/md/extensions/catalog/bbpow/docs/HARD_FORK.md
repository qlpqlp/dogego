# If BBPoW were added to Dogecoin consensus (hard fork)

This document is **design notes only**. DogeGo does **not** implement this on L1. The `dogego.bbpow` extension only verifies proofs off-chain on **testnet**.

## Short answers

| Question | Answer |
|----------|--------|
| Soft fork possible for “Bitcoin work **instead of** Scrypt”? | **No.** That expands valid blocks; legacy nodes reject → hard fork. |
| Soft fork if we only **add** a Bitcoin commitment **on top of** required Scrypt AuxPoW? | **Yes**, but Bitcoin-only ASICs still cannot produce Dogecoin blocks alone. |
| Reuse existing AuxPoW wire field? | **Yes for encoding**; validation must still change (SHA-256 vs Scrypt) → still a hard fork for OR semantics. |
| New ASIC for Bitcoin miners? | **No** for the Bitcoin lane - they keep SHA-256 ASICs. Scrypt AuxPoW still needs Scrypt ASICs. |

## Picture (yes, this is correct)

After a hard fork, **two hardware classes** can secure the **same** Dogecoin tip. Neither side needs new chips for its own lane:

```text
┌─────────────────────┐         ┌──────────────────────┐
│  SHA-256 ASIC       │         │  Scrypt ASIC         │
│  (Bitcoin miners)   │         │  (LTC/DOGE merge)    │
└─────────┬───────────┘         └──────────┬───────────┘
          │                                │
          │ mine BTC + embed               │ classic AuxPoW
          │ Dogecoin commitment            │ Litecoin parent
          ▼                                ▼
   BBPoW / Bitcoin-parent AuxPoW     Scrypt AuxPoW (today)
          │                                │
          └────────────┬───────────────────┘
                       ▼
              Dogecoin tip (after HF: OR of both)
```

How to read it:

- **Left:** Bitcoin miners keep mining Bitcoin as today. They add a small commitment (Dogecoin block hash) in the Bitcoin coinbase. Dogecoin (after HF) accepts that as a valid **Bitcoin-parent** AuxPoW-style proof. **Same SHA-256 ASICs.**
- **Right:** Litecoin/Dogecoin merge miners keep doing **classic Scrypt AuxPoW**. **Unchanged.**
- **Bottom:** A Dogecoin block is valid if **either** proof type passes (`OR`). Both lanes feed one chain tip.

Bitcoin’s protocol does not change. Only Dogecoin’s “is this PoW good enough?” rule widens.

---

## Don’t panic: what “hard fork” means here

“Hard fork” sounds scary. For this idea it is a **narrow** consensus change, not a rewrite of Dogecoin.

### What does **not** change

| Area | Still the same |
|------|----------------|
| Your addresses & keys | Same |
| Sending / receiving DOGE | Same transaction rules |
| UTXO model | Same |
| Scripts / OP codes (for this design) | Same |
| Bitcoin | Unchanged |
| Classic Litecoin AuxPoW | Still valid (the right side of the diagram) |
| Block header size (typical design) | Still 80-byte header + existing AuxPoW blob |

Users do not get new wallets. Coins do not move. Merchants do not learn a new address format.

### What **does** change (the simple core)

One question nodes ask today:

> “Does this AuxPoW’s **parent** pass **Scrypt** against Dogecoin’s target?”

After the hard fork, nodes ask:

> “Does the parent pass **Scrypt** (Litecoin path) **or** **SHA-256** (Bitcoin path) against the right target?”

That is the fork. Everything else is activation plumbing, difficulty fairness, and pool templates.

```text
BEFORE:  block valid ⇒ Scrypt AuxPoW OK
AFTER:   block valid ⇒ Scrypt AuxPoW OK  OR  Bitcoin-parent proof OK
```

Why it is still called a hard fork (honestly): old nodes that only know Scrypt will **reject** Bitcoin-backed blocks. Upgraded and non-upgraded nodes would disagree on the tip unless everyone upgrades. That is the definition - not “Dogecoin becomes a different coin overnight.”

### What else is needed so it stays fair (still small, but important)

| Extra | Why | Scary? |
|-------|-----|--------|
| Activation height | Everyone switches at the same block | Normal for any consensus upgrade |
| Dual difficulty (recommended) | So SHA-256 hashrate cannot instantly drown Scrypt (or reverse) | Design choice; multi-algo coins do this |
| Mining templates | Bitcoin pools embed the Dogecoin commitment | Pool software + Dogecoin mining RPC |
| Spec / tests | So Core, DogeGo, explorers agree | Engineering, not a new money type |

---

## Consensus change in detail (calm checklist)

### A. PoW validation (required - heart of it)

**Idea:** keep the AuxPoW *package* (parent header + coinbase + merkle proofs). Change only how parent work is verified.

| Step | Today | After HF |
|------|-------|----------|
| 1. Parse AuxPoW | Same | Same |
| 2. Merkle: coinbase ↔ parent header | Same | Same |
| 3. Commitment to Dogecoin block | Same idea | Same idea (Bitcoin coinbase commits to Dogecoin hash) |
| 4. Parent PoW | **Scrypt(parent)** vs Dogecoin bits | **If Litecoin-style:** Scrypt (unchanged). **If Bitcoin-style:** SHA-256d(parent) vs SHA-256-lane bits |

No need for a new mysterious “BBPoW field” if you reuse AuxPoW encoding. The rule change is step 4.

### B. Difficulty / chain work (strongly recommended)

Without this, the left lane (Bitcoin hashrate) could overwhelm the right lane.

| Piece | Role |
|-------|------|
| Scrypt-lane target | Keeps Digishield-style adjust for merge miners |
| SHA-256-lane target | Separate adjust for Bitcoin-backed blocks |
| Chain work accounting | How much “weight” each block adds when comparing tips |

Still Dogecoin difficulty math - extended, not replaced with Bitcoin’s rules wholesale.

### C. Activation

| Piece | Role |
|-------|------|
| Height or flagged start | Before height: old rules only. After: `OR` allowed |
| Version / signaling (optional) | Helps coordinate upgrades |

### D. Mining surface (operators, not “money rules”)

| Piece | Role |
|-------|------|
| `createauxblock` / submit path | Still works for Scrypt AuxPoW |
| Bitcoin commitment helper | So pools know what to put in the BTC coinbase |
| Explorers | Show “proof type: scrypt aux / bitcoin aux” |

### E. Out of scope for this HF idea

- Changing DOGE supply schedule (unless someone separately proposes that - not required)  
- Changing Bitcoin  
- Forcing Scrypt miners to buy SHA-256 ASICs (or reverse)  
- Rewriting the wallet  

---

## What part of the **code** changes (DogeGo map)

Dogecoin Core would need the same *logic* in C++. In **this** repo:

### 1. Heart - AuxPoW / PoW validation

| Area | Files (DogeGo) | What changes |
|------|----------------|--------------|
| AuxPoW check | `consensus/headers_validate.go` (`checkAuxPow`) | Branch: Scrypt parent **or** SHA-256 parent |
| Header connect | `consensus/validate_stored.go`, `node/headers_apply.go` | Honor activation; accept either proof |
| PoW helpers | `pow/pow.go`, `pow/merkle.go` | SHA-256 path for Bitcoin parent |
| Wire AuxPoW | `wire/auxpow.go`, `store/header_aux.go` | Often **reuse** blob; optional parent-algo tag |

```text
// today (simplified)
parentPow := Scrypt(parentHeader)
parentPow must meet dogecoin nBits

// after HF (simplified)
if bitcoinParent {
    parentPow := SHA256d(parentHeader)
    parentPow must meet sha256-lane target
} else {
    parentPow := Scrypt(parentHeader)  // Litecoin AuxPoW unchanged
    parentPow must meet scrypt-lane target
}
```

### 2. Difficulty / chain work

| Area | Files | What changes |
|------|-------|--------------|
| Next bits | `consensus/difficulty_export.go` (`NextBlockBits`), Digishield in `consensus/` | Per-lane retarget |
| Chain work | `node/` chain-work cache | Weight of each proof type |
| Params | `chain/params.go`, `consensus/dogeconsensus.go` | Activation height, lane limits |

### 3. Mining RPCs

| Area | Files | What changes |
|------|-------|--------------|
| Aux mining | `rpc/auxpow_mining.go` | Bitcoin-commitment path + existing Scrypt path |
| GBT / generate | `rpc/getblocktemplate.go`, `rpc/generate.go` | Templates / testnet |

### 4. Usually untouched

Wallet, keys, send, mempool scripts. Extension `dogego.bbpow` stays research until/unless L1 absorbs the rule.

### 5. Tests

`consensus/headers_validate_aux_test.go` + Bitcoin-parent vectors; dual-lane soak on testnet.

---

## Soft fork wish vs reality

- Same AuxPoW **bytes**: OK as packaging.  
- Soft fork with **OR**: no (legacy still Scrypt-only).  
- Soft fork with **AND**: yes, but Bitcoin-only ASICs still cannot solo-produce DOGE.

See [PLAIN_ENGLISH.md](PLAIN_ENGLISH.md).
