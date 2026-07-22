# BBPoW / CAuxPoW protocol sketch (research)

**Status:** Experimental. Implemented as a DogeGo **extension** verifier only.  
**Consensus:** Not Dogecoin L1. Making Bitcoin SHA-256 work a valid Dogecoin block proof is a **Dogecoin hard fork** (see below). A soft fork that lets Bitcoin-only hashrate secure Dogecoin **without** Scrypt work is not possible under Bitcoin/Dogecoin soft-fork rules.

## Naming

Avoid calling this "AuxPoW" when the parent is Bitcoin. Prefer:

- **BBPoW** - Bitcoin-Backed Proof-of-Work  
- **CAuxPoW** - Cross-Algorithm Auxiliary Proof-of-Work  
- **EPoW** - External Proof-of-Work  

Classic **AuxPoW** (Litecoin → Dogecoin) is Scrypt merge-mining and stays on L1 today.

## ASICs: do Bitcoin miners need two machines?

| Miner today | Hardware | Can they BBPoW-style help Dogecoin? | Can they classic AuxPoW Dogecoin? |
|-------------|----------|-------------------------------------|-----------------------------------|
| Bitcoin pool / SHA-256 ASIC | SHA-256 only | **Yes (research HF design):** same SHA-256 ASIC mines Bitcoin; embed Dogecoin commitment in the Bitcoin coinbase. No Scrypt ASIC required for the *Bitcoin* lane. | **No** - AuxPoW parent PoW is Scrypt |
| Litecoin / Dogecoin merge miner | Scrypt ASIC | Not via Bitcoin BBPoW (wrong algo) | **Yes** - existing AuxPoW |
| One box, both algos | Need **both** ASIC types | Only if you run both stacks | Scrypt ASIC for AuxPoW |

**Bottom line:** Bitcoin SHA-256 ASICs cannot run Scrypt. Scrypt ASICs cannot run Bitcoin SHA-256. BBPoW’s point is that Bitcoin miners keep using **the same SHA-256 ASICs they already have**; they do **not** buy Dogecoin ASICs. They are not “mining Dogecoin Scrypt” - they are proving Bitcoin SHA-256 work that Dogecoin (under a hard fork) would count.

They are not mining “both chains with one chip” in the AuxPoW sense. They mine Bitcoin as usual; Dogecoin would *recognize* that work. Litecoin/Scrypt merge-miners remain a separate class of hardware.

## Goal

Allow Dogecoin to be secured by both:

* Litecoin miners (existing Scrypt AuxPoW)
* Bitcoin miners (SHA-256 external PoW)

while leaving **Bitcoin** unchanged.

## Soft fork vs hard fork (why soft fork fails for OR)

### Soft fork definition (Bitcoin-style)

A soft fork **tightens** rules: some previously valid blocks become invalid. Old nodes still accept blocks produced under the new rules (they look valid to legacy checks).

### What BBPoW-as-consensus needs

```text
Dogecoin block is valid if:

    ValidScryptAuxPoW()
        OR
    ValidBitcoinProof()     // SHA-256 parent work
```

That **expands** the set of valid blocks. Legacy nodes still require Scrypt/AuxPoW (`CheckAuxPow` uses **parent Scrypt** against child `nBits`). A Bitcoin-parent AuxPoW blob fails Scrypt PoW on those nodes → they **reject** the block → chain split → **hard fork**.

### Soft-fork shaped alternatives (and why they don’t replace hard fork)

| Idea | Soft fork? | Useful for Bitcoin hashrate? |
|------|------------|------------------------------|
| **OR** accept SHA-256 proofs | No (hard fork) | Yes |
| **AND** require Scrypt AuxPoW *plus* optional Bitcoin commitment | Yes (extra commitment only on upgraded miners) | **No** - Bitcoin-only miners still need Scrypt work |
| Hide SHA-256 proof in witness / extension space while header still has fake Scrypt | Legacy still needs a **valid** Scrypt AuxPoW on the wire | Still need Scrypt ASIC work |
| Version bits that old nodes treat as anyone-can-spend style tricks | PoW is not a script rule; header PoW is checked by all nodes | Does not apply cleanly |

**Conclusion:** Reusing the AuxPoW *bytes* does not make this a soft fork. Soft fork cannot admit “Bitcoin work instead of Scrypt work” while old nodes keep today’s AuxPoW rules.

## Can we reuse the exact AuxPoW field (no new mining field)?

**Wire layout: mostly yes. Validation: no (still a hard fork).**

Dogecoin already stores AuxPoW after an auxpow-version header (`CAuxPow`: parent coinbase, merkle branches, parent 80-byte header). A Bitcoin-backed design could **reuse that same structure**:

- Parent header = Bitcoin header (not Litecoin)
- Parent coinbase = Bitcoin coinbase with merge-mining / commitment bytes
- Merkle / chain branches = same shape as today

**Mining software** could keep producing “an AuxPoW attachment” instead of inventing a parallel `BitcoinProof` field - good for familiarity.

But today’s consensus does this (DogeGo / Core-aligned):

```text
parentPow := Scrypt(parentHeader)
parentPow must meet child nBits
```

Bitcoin’s header PoW is **SHA-256d**, not Scrypt. A Bitcoin parent **fails** current `CheckAuxPow`. Changing that check to:

```text
if parent_is_bitcoin_style:
    SHA256d(parentHeader) meets mapped target
else:
    Scrypt(parentHeader) meets target   // Litecoin AuxPoW unchanged
```

…is still a **consensus rule change** (and usually a **hard fork** if OR’d). You avoided a *new wire field*, not a fork.

This extension currently uses an explicit research `BitcoinProof` JSON for clarity. A hard-fork design could instead specify “AuxPoW with SHA-256 parent validation” and keep one on-disk aux field.

## Validity sketch (future hard fork - not this extension)

```text
Dogecoin block is valid if:

    ValidScryptAuxPoW()          // Litecoin parent, Scrypt (unchanged)
        OR
    ValidBitcoinAuxPoW()         // Bitcoin parent, SHA-256d (new rule)
```

Optional: dual difficulty / separate lanes so one algo cannot starve the other.

## What a consensus hard fork would involve

Not implemented in DogeGo L1. Checklist if the community ever designed one:

1. **BIP / Dogecoin DIP** - precise validation, activation height, naming (BBPoW vs “AuxPoW v2”).  
2. **Parent PoW switch** - SHA-256d for Bitcoin parent; keep Scrypt path for Litecoin.  
3. **Work / difficulty** - map Bitcoin work ↔ Dogecoin work; likely **separate adjusters** per lane.  
4. **Commitment format** - merge-mining header + chain merkle root (classic) vs `BBPoW||hash`; chain ID rules.  
5. **Bitcoin context** - how much Bitcoin chain tip Dogecoin nodes must verify (headers-only? checkpoints?).  
6. **Bitcoin reorgs** - what happens when the Bitcoin parent is reorged.  
7. **Mining templates** - GBT / createauxblock style for Bitcoin pools (coinbase commitment).  
8. **Activation** - height or flag day; **all** economic nodes must upgrade (hard fork).  
9. **Incentives** - Bitcoin pools get no native BTC reward for Dogecoin commitments; need pool policy / side payments.  
10. **Security analysis** - majority SHA-256 vs Scrypt dominance, rental attacks, difficulty cliffs.  
11. **Testnet soak** - long dual-algo testnet (this extension is only a verifier sandbox).  
12. **Wallet / explorers / pools** - display proof type, reject ambiguous tips during upgrade.

Bitcoin protocol stays unchanged; only Dogecoin (and tooling) changes.

## BitcoinProof (v1) - extension research format

| Field | Meaning |
|-------|---------|
| `doge_block_hash` | Dogecoin block id (display hex) |
| `bitcoin_header_hex` | 80-byte Bitcoin header |
| `coinbase_tx_hex` | Bitcoin coinbase containing commitment |
| `merkle_branch_hex` | Merkle path from coinbase txid to header merkle root |
| `merkle_index` | Tx index (usually 0) |
| `bitcoin_context_hex` | Optional parent headers (each must PoW + link) |

### Commitment (research)

```text
BBPoW || doge_block_hash_le32
```

A hard-fork AuxPoW-reuse design might instead keep Litecoin-style `fabe'mm'` + chain merkle root in the Bitcoin coinbase.

### Checks performed by this extension

1. Bitcoin header SHA-256d meets `nBits` (research may allow easy targets; mainnet check is separate).  
2. Commitment present for the claimed Dogecoin hash.  
3. Coinbase merkle-proves into the header merkle root.  
4. Optional context headers form a parent chain with valid PoW.

## Difficulty (open research)

* Does 1 unit of Bitcoin work equal 1 unit of Dogecoin work?  
* Separate difficulty targets / adjustments per algorithm?  
* How to prevent one mining class from overwhelming the other?  

## Extension boundary

| In extension | Not in extension |
|--------------|------------------|
| Proof verify RPC | Changing `consensus/` validation |
| Dual-lane research stats | Mainnet enable / soft-fork activation |
| Commitment builder | Rejecting or accepting L1 blocks |
| Testnet only | Shipping as default consensus |

See also [USER_GUIDE.md](USER_GUIDE.md) and [HARD_FORK.md](HARD_FORK.md).
