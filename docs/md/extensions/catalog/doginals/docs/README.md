# Doginals / DRC-20 L2 (`dogego.doginals`)

**v0.7.0** — experimental DogeGo extension that:

1. **Indexes L1** classic **P2SH Doginals** (apezord/booktoshi), Ord **envelopes**, and **OP_RETURN** (existing + future)  
2. **Mints on L2 by default** when the extension is enabled — **tokens, images, and files** with wallet `signmessage` proofs  
3. **Does not mint P2SH on L1** — P2SH is **index-only**; optional legacy L1 path is OP_RETURN via `inscribe`  
4. **Classifies media** as token / image / text / json / file and serves content for display  
5. **Tracks address balances** and gossips signed L2 mints via `doginals-v1`  
6. Ships a **wizard UI** with file/image picker  

**Does not change Dogecoin consensus.**

## Docs

| File | Purpose |
|------|---------|
| [USER_GUIDE.md](USER_GUIDE.md) | Install, mint UX, HTTP API, RPC |
| [PROTOCOL.md](PROTOCOL.md) | L1 index + signed L2 mint wire format |

## Protocol references

- [apezord/doginals](https://github.com/apezord/doginals) — original P2SH Doginals protocol (indexed)  
- [booktoshi/doginals](https://github.com/booktoshi/doginals) — community inscriber tooling  
