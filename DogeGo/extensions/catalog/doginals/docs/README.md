# Doginals / DRC-20 L2 (`dogego.doginals`)

**v0.4.0** — experimental DogeGo extension that:

1. **Indexes L1** OP_RETURN / data-carrier outputs (DRC-20, doginal-like, generic data)  
2. **Tracks address balances** (wallet read shape)  
3. **Exposes a wallet HTTP API** at `/api/ext/dogego.doginals/v1/*` (extension-owned `httphandle`; host only proxies `/api/ext/{id}/…`)  
4. **Stores L2 assets** off-chain and **syncs** them via `doginals-v1` (`dinv` / `getdasset` / `dasset`)  
5. **Mints** DRC-20 on L1 (wallet RPC) or experimentally off-L1 (`mintl2`)  
6. Ships a **wizard UI**: Setup → Sync → Create → Wallet API  

**Does not change Dogecoin consensus.**

## Docs

| File | Purpose |
|------|---------|
| [USER_GUIDE.md](USER_GUIDE.md) | Install, wizard, HTTP API, RPC |
| [PROTOCOL.md](PROTOCOL.md) | Overlay wire + storage model |

## Build

```powershell
.\build.ps1
# → dist/doginals-universal.zip (+ doginals.zip copy) and sha256
```

Refresh catalog hashes from `DogeGo/`:

```powershell
.\scripts\build_extensions_catalog.ps1
```
