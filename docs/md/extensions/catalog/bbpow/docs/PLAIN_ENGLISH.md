# How BBPoW actually works (plain language)

## The goal you want

> Bitcoin miners keep the **same SHA-256 ASICs**, change almost nothing, and somehow also secure / produce **Dogecoin**, using DogeGo only, **without any fork**.

That goal has **two parts**. One is possible as a **toy / research** feature. The other is **not** possible on real Dogecoin without changing consensus (a hard fork).

---

## How Dogecoin AuxPoW works today (important)

Think of AuxPoW as:

```text
Parent chain does real PoW  →  Child chain accepts that work
```

For Dogecoin today:

```text
Litecoin (Scrypt ASIC)  →  Dogecoin accepts Litecoin work as AuxPoW
```

Dogecoin nodes check roughly:

1. There is an AuxPoW blob (parent header + coinbase + merkle proofs).  
2. The **parent header’s Scrypt hash** is hard enough for Dogecoin’s rules.  
3. The coinbase commits to this Dogecoin block.

**Bitcoin cannot be that parent under today’s rules**, because:

- Bitcoin PoW = **SHA-256**  
- Dogecoin AuxPoW parent check = **Scrypt**  

A Bitcoin ASIC never produces a Scrypt hash. So a Bitcoin block, plugged into the AuxPoW field, **fails** current validation. Every normal Dogecoin node (Core, DogeGo L1, etc.) rejects it.

### “Any coin can use Dogecoin AuxPoW”

What people usually mean:

- **Other Scrypt coins** can be designed like Dogecoin: “accept Litecoin (or another Scrypt parent) AuxPoW.”  
- That still requires **Scrypt** parent work.

It does **not** mean:

- Any coin (including Bitcoin) can AuxPoW into Dogecoin, or  
- Dogecoin can accept Bitcoin work with zero rule changes.

Algorithms must match for classic AuxPoW.

---

## What the DogeGo **extension** does (BBPoW today)

```text
Bitcoin miner (SHA-256)
        │
        │  (optional) puts a Dogecoin hash into Bitcoin coinbase
        ▼
   Extension RPC: “is this Bitcoin proof well-formed?”
        │
        ▼
   Answer stored / shown in DogeGo UI  ← research only
```

The extension:

- Runs **beside** the node (testnet).  
- Can **check** a Bitcoin header + commitment + merkle proof.  
- Does **not** make that into a Dogecoin block on the chain.  
- Does **not** pay DOGE.  
- Does **not** change what peers send you as the tip.

So as an extension, BBPoW is a **verifier and lab notebook**, not “Bitcoin is mining Dogecoin.”

Your node still only accepts normal Dogecoin blocks (Scrypt / Litecoin AuxPoW).

---

## What “Bitcoin miners mine Dogecoin with no hardware change” would require

For Bitcoin hashrate to create **real Dogecoin blocks** that wallets/exchanges accept:

```text
Dogecoin block is valid if:

    (today) Scrypt AuxPoW from Litecoin
        OR
    (new)   SHA-256 proof from Bitcoin
```

That second `OR` is a **new consensus rule**.

| Approach | Hardware change for BTC miners? | Fork? | Real DOGE? |
|----------|----------------------------------|-------|------------|
| Extension only (what we shipped) | None | No fork of L1 | **No** - only a research check |
| Soft fork “OR Bitcoin work” | None | Not a soft fork; legacy nodes reject | Would split the network |
| Hard fork “OR Bitcoin work” | None (same SHA-256 ASICs) | **Yes, Dogecoin hard fork** | Yes, after everyone upgrades |
| Soft fork “AND Bitcoin commitment + still need Scrypt” | Still need Scrypt somewhere | Soft-forkable | Bitcoin-only ASICs still can’t solo-produce DOGE |

**Without any fork:** Bitcoin miners cannot produce blocks that the **existing** Dogecoin network accepts. Physics of consensus: old rules still require Scrypt AuxPoW.

**“Only using DogeGo”** does not remove that. If only DogeGo accepted Bitcoin proofs and Core did not, you would have **two different Dogecoins** (a de facto fork / alt tip), not “Dogecoin with no fork.”

---

## Picture

### Today (real Dogecoin)

```text
Scrypt ASIC → Litecoin-style AuxPoW → Dogecoin L1 accepts → DOGE
SHA-256 ASIC → Bitcoin only          → Dogecoin L1 ignores
```

### Extension (experimental)

```text
SHA-256 ASIC → Bitcoin + commitment → DogeGo extension says “proof looks OK”
                                    → Dogecoin L1 still ignores
```

### After a Dogecoin hard fork (not implemented)

```text
Scrypt ASIC  → AuxPoW     ─┐
                           ├→ Dogecoin L1 accepts either → DOGE
SHA-256 ASIC → BTC proof  ─┘
```

Bitcoin hardware unchanged; **Dogecoin software rules** changed.

---

## Reusing the AuxPoW field

You can **reuse the same AuxPoW bytes** (parent header + coinbase + merkle) and put a **Bitcoin** header in the parent slot so miners don’t invent a new blob format.

That only helps **encoding**. Nodes must still be taught:

> “If parent is Bitcoin-style, check SHA-256, not Scrypt.”

Teaching that is the hard fork. Same field ≠ no fork.

---

## One sentence summary

- **Extension:** DogeGo can *study* Bitcoin commitments; it does not make Bitcoin mine Dogecoin.  
- **No fork:** Impossible for Bitcoin ASICs alone to create universally accepted DOGE.  
- **Same ASICs + real DOGE:** Needs a **Dogecoin hard fork** (OR accept SHA-256 proofs), not a soft fork and not extension-only.

See also [PROTOCOL.md](PROTOCOL.md) and [HARD_FORK.md](HARD_FORK.md).
