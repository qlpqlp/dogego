# Doginals / DRC-20 L2 (`dogego.doginals`)

**v0.8.0** - experimental DogeGo extension that:

1. **Indexes L1** classic **P2SH Doginals** (apezord/booktoshi), Ord **envelopes**, and **OP_RETURN** (existing + future)
2. **Mints only on L2** when the extension is enabled - **tokens, images, files, and Ordinals** (`ord` envelopes) with wallet `signmessage` proofs
3. **Does not mint on L1** (no P2SH / Ordinals / OP_RETURN builder). On-chain inscriptions are index-only
4. **Classifies media** as token / image / text / json / file and serves content for display
5. **Tracks address balances** and gossips signed L2 mints via **permissionless** `doginals-v1` P2P
6. Ships a **wizard UI** with file/image picker

**Does not change Dogecoin consensus.**

## Docs

| File | Purpose |
|------|---------|
| [USER_GUIDE.md](USER_GUIDE.md) | Install, L2 mint UX, how L1 index + L2 gossip work, HTTP API, RPC |
| [PROTOCOL.md](PROTOCOL.md) | L1 index + signed L2 mint wire format + permissionless sync |

## Protocol references

- [apezord/doginals](https://github.com/apezord/doginals) - original P2SH Doginals protocol (indexed)
- [booktoshi/doginals](https://github.com/booktoshi/doginals) - community inscriber tooling
