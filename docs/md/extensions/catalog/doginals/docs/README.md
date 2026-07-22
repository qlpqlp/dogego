# Doginals / DRC-20 L2 (`dogego.doginals`)

Experimental DogeGo extension that:

1. **Indexes L1** OP_RETURN / data-carrier outputs (DRC-20 JSON, doginal-like markers, generic data)
2. **Stores L2 assets** off-chain (NFT / token / image / collection metadata + URI or inline content)
3. **Syncs L2 assets** with other DogeGo peers via overlay protocol `doginals-v1` (`dinv` / `getdasset` / `dasset`)
4. Exposes a **friendly dashboard panel** (chips, tools, gallery actions) - same UI contract as ZK L2

**Does not change Dogecoin consensus.** L1 indexing is observe-only. Creating L2 assets does not write to the chain unless you separately broadcast your own transactions.

## Docs

| File | Purpose |
|------|---------|
| [USER_GUIDE.md](USER_GUIDE.md) | Install, enable, RPC, dashboard |
| [PROTOCOL.md](PROTOCOL.md) | Overlay wire + data model |
