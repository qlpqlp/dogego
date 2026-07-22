# SegWit status (Dogecoin Core-aligned)

DogeGo mirrors **Dogecoin Core**: SegWit (BIP141/143/147) **machinery is present**, mainnet **rules are disabled**. This is not a DogeGo-only choice and is **not** a pending BIP9 softfork waiting on miner signaling.

## What exists today

| Layer | Behavior |
|-------|----------|
| Wire / RPC size | Legacy + witness serialization; BIP141 weight/vsize helpers for decoding |
| Consensus | `IsWitnessEnabled` always **false** (Core `validation.cpp`) |
| BIP9 | Deployment named `"segwit"` with **`Timeout: 0`** (Core “Disabled”; never leaves defined via timeout) |
| Mempool / P2P | Witness txs rejected (`bad-txns-witness-not-supported` / admission reject) |
| Wallet | `addwitnessaddress` returns Core-shaped “not enabled on network”; `getnewaddress` is legacy-only |
| Script tests | Core `script_tests.json` **WITNESS** rows skipped by design |

Primary code: `consensus/witness_policy.go`, `consensus/versionbits.go`, `consensus/tx_check.go`, `node/p2p_tx.go`.

## CLTV vs SegWit (historical sequencing)

[Dogecoin Core issue #1760](https://github.com/dogecoin/dogecoin/issues/1760) (native bech32) notes that SegWit **code** shipped in the 1.14 line but was **not activated** because **CLTV (BIP65) was needed first**.

On mainnet today:

- **CLTV is already activated** at height **3,464,751** (`BIP65Height` in Core and DogeGo).
- SegWit remains **hard-disabled** (`IsWitnessEnabled` → false, BIP9 `nTimeout = 0`).

So CLTV is **not** an open blocker for DogeGo; both projects still keep SegWit off until Core changes activation parameters.

## Bech32 / native SegWit addresses

**Not implemented in Dogecoin Core** yet (open enhancement on [#1760](https://github.com/dogecoin/dogecoin/issues/1760), milestone 1.21). DogeGo does **not** ship `doge1…` receive ahead of Core. P2SH-wrapped witness helpers in Core (`addwitnessaddress`) are also non-functional while SegWit is disabled.

## Softfork education (if Core later activates SegWit)

A softfork **tightens** consensus rules. Old valid blocks stay valid; some previously accepted transactions/blocks become invalid under the new rules.

If Dogecoin Core schedules and activates SegWit (BIP9 or buried height, whatever Core publishes):

| Actor | If they **do not** upgrade | If they **upgrade** |
|-------|----------------------------|---------------------|
| **Full nodes** | May still follow the heaviest valid chain under *old* rules, but can accept/relay txs that upgraded peers reject; risk of **false confirmations** or wallet confusion on witness spends | Enforce witness rules; reject invalid witness txs; stay compatible with upgraded miners |
| **Miners / pools** | May produce blocks that upgraded nodes **reject** → **orphans / lost rewards** | Enforce witness commitments and policy; can include valid witness txs |
| **Wallets / services** | Legacy P2PKH/P2SH continue; cannot safely use native witness outputs until upgraded | Can send/receive under the new rules once addresses and tooling exist |

**DogeGo policy:** enable SegWit consensus **only when Core’s activation parameters land** in `chainparams` / `IsWitnessEnabled`. Solo activation in DogeGo would be a **mainnet consensus fork ahead of Core** and is out of scope ([INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md)).

## Related docs

- [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md): protocol lock  
- [CORE_PARITY_GAPS.md](CORE_PARITY_GAPS.md): out-of-scope witness relay  
- [ROADMAP.md](../ROADMAP.md): Phase 4 MVP: SegWit disabled like Core  
- Upstream: [dogecoin/dogecoin](https://github.com/dogecoin/dogecoin), [issue #1760](https://github.com/dogecoin/dogecoin/issues/1760)
