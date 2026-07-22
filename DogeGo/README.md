# DogeGo

Much Faster Full Dogecoin Node - beta Go implementation with mainnet IBD, reboot testnet, loopback web UI, and Core-shaped JSON-RPC.

**Canonical repository:** [github.com/qlpqlp/dogego](https://github.com/qlpqlp/dogego) (app lives in this `DogeGo/` directory; website at [dogego.org](https://dogego.org)). DogeGo moved here from the former `dogecoin/DogeGo` path in the [Dogecoin Core](https://github.com/dogecoin/dogecoin) tree.

## License

DogeGo is [MIT licensed](LICENSE). Copyright (c) 2026 Paulo Vidal and Dogecoin Foundation.
Implementation is informed by [Dogecoin Core](https://github.com/dogecoin/dogecoin) and [Bitcoin Core](https://github.com/bitcoin/bitcoin) (both MIT); see `LICENSE` for upstream attribution.

**Beta:** please run DogeGo, try the workflows you care about, and report what needs tuning. Core-compatible consensus, RPC, wallet migration, mining, and a loopback web UI ship today; storage is DogeGo’s native Go layout (not Core LevelDB). See **`docs/OVERVIEW.md`** and **`ROADMAP.md`**.

### Mainnet full node (beta)

DogeGo targets a **standalone** mainnet full node (headers + bodies, P2P, RPC, web UI) with Core-aligned behavior on consensus-critical paths where implemented. It is **not** production-certified - see **`docs/STANDALONE_FULLNODE_ACCEPTANCE.md`**. **Mainnet consensus follows Dogecoin Core** (no protocol forks; see **`ROADMAP.md`** **Dogecoin protocol lock**). For exchange-grade deployments, still prefer **Dogecoin Core** until the acceptance matrix is complete.

**Practical options today:**

- **DogeGo** mainnet IBD + operator tooling: **`docs/STANDALONE_NODE_QUICKSTART.md`**, **`docs/CORE_OPERATOR_RUNBOOK.md`**, `go run ./cmd/dogego cert offline` (see **`scripts/README.md`**; Windows also `.\scripts\operator_workflow_cert.ps1`).
- **Dogecoin Core** when you need full script corpus parity, `wallet.dat`, or established operator SLOs.
- **Automation in Go:** call **`dogecoin-cli`** / JSON-RPC from Go, or use DogeGo’s JSON-RPC where methods are implemented (`docs/RPC.md`).
## Long-term project (full node)

DogeGo is being grown **incrementally** toward a Dogecoin-compatible node in Go.

- **`ROADMAP.md`** - phased plan (P2P → serialization → sync → consensus → store → mempool → RPC, web, security, PQC alignment).
- **`docs/DEVELOPER_GUIDE.md`** - contributor map: where every package and config layer lives.
- **`docs/CHAIN_PARAMETERS.md`** - mainnet / reboot testnet / DNS seeds / fixed peers / genesis / checkpoints (not a single file; editing checklist).
- **`docs/STANDALONE_NODE_QUICKSTART.md`** - build, first run, post-aux upgrade checks, certification scripts.
- **`docs/DOCUMENTATION.md`** - documentation index (operator + integrator); sync with web **Docs** tab.
- **`docs/INTEGRATION.md`** - connect external apps (JSON-RPC, auth, examples).
- **`docs/RPC.md`** / **`docs/WALLET.md`** - workflow indexes (per-method cookbooks: ROADMAP Phase 12).
- **`docs/OVERVIEW.md`** - what DogeGo does today vs Core, analytics, honest scope (performance, wallet, PQ).
- **`docs/WEB_UI.md`** - dashboard tabs including **Docs**, **Features** (17 live operator-cert gates, mining + PQ probes), Send/Receive/History PQ carrier mode.
- **`docs/ARCHITECTURE.md`** - target package layout and design rules.
- **Reserved packages** - `node/`, `p2p/`, `consensus/`, `store/`, `rpc/` (skeleton `doc.go` only until each phase starts).

Use **`dogego node`** for a **minimal experimental process** (headers sync + optional RPC). It is **not** a safe replacement for Core on untrusted networks.

Running just `dogego` (or `dogego.exe`) is equivalent to `dogego node`.

```text
go run ./cmd/dogego genesis
go run ./cmd/dogego ping [-host H] [-port P]
go run ./cmd/dogego node -datadir DIR -peer HOST:PORT [-network reboottestnet|mainnet] [-rpc 127.0.0.1:18556]
```

## Windows binary (testnet-oriented client)

From this directory:

```powershell
go build -o dogego.exe ./cmd/dogego
```

Cross-compile from Linux/macOS:

```text
GOOS=windows GOARCH=amd64 go build -o dogego.exe ./cmd/dogego
```

## Parameters

Reboot testnet params align with Dogecoin Core `CTestNetParams`: P2P magic `fd d4 dc e1`, default port **44556**, genesis **block hash** `d5d619f8…` (`GetHash()`), and Dogecoin-style **scrypt** (`GetPoWHash()`) on the 80-byte header. See [`docs/CHAIN_PARAMETERS.md`](docs/CHAIN_PARAMETERS.md) and [`chain/README.md`](chain/README.md).

## Repository root

Clone the canonical repo and build from this directory:

```text
git clone https://github.com/qlpqlp/dogego.git
cd dogego/DogeGo
go build -o dogego ./cmd/dogego    # or dogego.exe on Windows
```

Release binaries and in-app update checks use [github.com/qlpqlp/dogego/releases](https://github.com/qlpqlp/dogego/releases).
