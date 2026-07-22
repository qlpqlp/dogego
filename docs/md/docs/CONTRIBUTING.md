# Contributing to DogeGo

## Before opening a PR

1. **Protocol lock** - DogeGo does **not** change Dogecoin mainnet consensus rules (no protocol forks). Consensus/P2P changes must match [Dogecoin Core](https://github.com/dogecoin/dogecoin) or document the difference in [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md). Run `go run ./cmd/dogego cert offline` (or `scripts/cert_offline_prerequisites.{ps1,sh}`) when touching consensus, store recovery, or mempool policy.
2. **Consensus / P2P changes** - Match Core behavior or document the difference in [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md).
3. **New JSON-RPC** - Register in `rpc/dispatch.go` (`SupportedMethods` + `case`), add a line in `rpc/help.go`, and extend `rpc/dispatch_sync_test.go` / `rpc/help_test.go` coverage.
4. **Documentation** - Update [ROADMAP.md](../ROADMAP.md) checkboxes, [docs/RPC.md](RPC.md) or [docs/WALLET.md](WALLET.md) when workflows change, and `ui/docs_index.go` if the Docs tab narrative changes (`ui/docs_index_test.go` must pass).
5. **Tests** - Run `go test ./...` from the `DogeGo/` directory (skip long `node` stress tests if needed: `go test ./... -short` where supported).

## Build

```powershell
cd DogeGo
go build -trimpath -o dogego.exe ./cmd/dogego
```

## RPC parity checklist

- [ ] `help("method")` text added
- [ ] Core-shaped error codes where applicable (`-31` P2P disabled, `-8` invalid parameter, etc.)
- [ ] `dogego_*` fields only for DogeGo-specific diagnostics (not replacements for Core fields)
