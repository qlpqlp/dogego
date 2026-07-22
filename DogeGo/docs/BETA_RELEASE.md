# DogeGo public beta - operator checklist

DogeGo is **beta** (`version.Beta = true`). It targets Core-compatible behavior but is **not** standalone-certified. See [CORE_PARITY_GAPS.md](CORE_PARITY_GAPS.md) for the full matrix.

## Before you ship / announce

1. **Offline gate (required)**  
   ```bash
   go run ./cmd/dogego cert offline
   ```  
   Must exit `0` (CI uses the same gate).

2. **P2P port**  
   Stop **Dogecoin Core** on the same network before starting DogeGo. Mainnet P2P is **22556** - only one process can bind it. The setup wizard now **blocks** start when the port is taken.

3. **Solo operator cert**  
   On Overview, **Operator cert**, use **solo** pass (`solo_ok`) for beta sign-off. Optional Core compare (`core_rpc_addr`) is not required.

4. **Wallet.dat migration**  
   Pool-only rows in Core `wallet.dat` **cannot** recover private keys. Matched pool pubkeys replay into HD `wallet.json`. See [OPERATOR.md](OPERATOR.md).

5. **Mainnet IBD**  
   Initial sync takes **days**, not hours. Connect lag after restart is normal until stored bodies replay; connect catch-up runs automatically.

## What works for beta users

- Full-node sync (headers + block bodies + UTXO replay)
- Web dashboard (wallet send/receive, BlockStep, Analytics)
- JSON-RPC (180+ methods, Core-shaped errors on operator paths)
- Native `wallet.dat` import (with passphrase for encrypted wallets)
- Offline mempool policy corpus (58/58) and legacy script tests (1059/1059)

## What is not beta-ready

- Production exchange / custody use (use **Dogecoin Core**)
- Full Core keypool file semantics 1:1
- Milestone B/D/E live soak on mainnet with Core side-by-side
- Litecoin parent chain for AuxPoW deep reorgs

## Quick smoke test

```powershell
go run ./cmd/dogego cert offline
.\scripts\watch_sync.ps1   # optional during IBD
# Open http://localhost:2013 - Overview solo cert 13/13 when caught up
```
