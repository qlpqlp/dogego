# DogeGo documentation index

Goal: **every operator and integrator** can learn how DogeGo works, how to run it, how to call JSON-RPC, and how to connect external applications - from the **web dashboard** and from this `docs/` folder.

DogeGo is **not** Dogecoin Core. Always read [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md) before assuming Core behavior or file formats. **Mainnet consensus rules follow Core** (no protocol forks); see [ROADMAP.md](../ROADMAP.md) **Dogecoin protocol lock**.

## Web dashboard

Open **Docs** in the dashboard (`http://localhost:2013/` by default), or read the same guides online at [dogego.org/guide/](https://dogego.org/guide/) (no install). Pick a section for your role (new operator, integrator, or contributor), click a file link, then follow links inside the document. They stay in the viewer. Optional Dogecoin Core compare is never required for solo operation.

## Status

| Area | In `docs/` | Web dashboard | Notes |
|------|------------|---------------|--------|
| **Protocol lock** | [ROADMAP.md](../ROADMAP.md), [SECURITY.md](SECURITY.md) | Guide + Features | No mainnet consensus forks; Core is the reference |
| Operator setup | [OPERATOR.md](OPERATOR.md) | Guide + Docs tabs | Datadir, networks, security, **`ibd_optimize`** / **`dbcache`** |
| Mainnet / testnet runbook | [CORE_OPERATOR_RUNBOOK.md](CORE_OPERATOR_RUNBOOK.md) | Docs tab | IBD, header recovery, mining, indexes |
| Web UI | [WEB_UI.md](WEB_UI.md) | Docs tab | Tabs, APIs, setup wizard |
| External apps | [INTEGRATION.md](INTEGRATION.md) | Docs tab | JSON-RPC, auth, examples |
| RPC workflows | [RPC.md](RPC.md), [RPC_CONSOLE_TUTORIAL.md](RPC_CONSOLE_TUTORIAL.md) | Console + Features tab | Full cookbook: `GET /api/rpc/cookbook` |
| Wallet | [WALLET.md](WALLET.md) | Guide + Send/Receive | PSBT, import, bumpfee, PQ commitments/carrier |
| **Post-quantum** | [WALLET.md](WALLET.md) § PQ flags, [WEB_UI.md](WEB_UI.md), [RACCOON_G_BUILD.md](RACCOON_G_BUILD.md), [CREDITS.md](CREDITS.md) | Features → PQ probe | Raccoon-G = Foundation in-tree port by [Ed Tubbs](https://github.com/edtubbs); GitHub Releases use **native** CGO runners (**not** cross-compile); `dogego cert pq` |
| **Plain HTTP / DogeBox** | [WEB_UI.md](WEB_UI.md) § `-notls`, [SECURITY.md](SECURITY.md), [OPERATOR.md](OPERATOR.md) | Settings → Local HTTPS | `-notls` / `DOGEGO_NO_TLS=1` skips cert + CA install |
| Architecture | [ARCHITECTURE.md](ARCHITECTURE.md) | Docs tab | Packages, data flow |
| **Contributors** | **[DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)** | Docs tab | Repo map, how to add RPC/consensus |
| **Chain / networks** | **[CHAIN_PARAMETERS.md](CHAIN_PARAMETERS.md)** | Docs tab | Mainnet, testnet, seeds, genesis, checkpoints |
| Native storage | [OPERATOR.md](OPERATOR.md) § Data layout | Overview / Guide | `headers.bin`, `rawblocks/`, `indexes/tx/` |
| Security | [SECURITY.md](SECURITY.md) | Guide | Threat model checklist |
| Parity / gaps | [ROADMAP.md](../ROADMAP.md), [CORE_PARITY_GAPS.md](CORE_PARITY_GAPS.md) | Features tab | Engineering checklist + Core backlog |
| Standalone readiness | [STANDALONE_FULLNODE_ACCEPTANCE.md](STANDALONE_FULLNODE_ACCEPTANCE.md) | - | Checklist vs Core |
| Quick standalone start | [STANDALONE_NODE_QUICKSTART.md](STANDALONE_NODE_QUICKSTART.md) | - | Build, post-aux upgrade, scripts |
| **Offline certification** | [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) § Offline | Features → Certification | `dogego cert offline`, `wallet-import`, `operator`, `pq` |
| **Extensions & ZK L2** | [EXTENSIONS.md](EXTENSIONS.md) | Extensions sidebar + Docs tab | Catalog/zip install, `dogego.zkl2`, wasm/subprocess; no wallet access |

Track documentation work in **ROADMAP.md → Phase 12 - Documentation & integrator UX**.

## Quick links

- **Foundations:** [BITCOIN_WHITEPAPER.md](BITCOIN_WHITEPAPER.md) - Satoshi Nakamoto (2008); also in Docs tab → Bitcoin white paper
- **Chain / networks:** [CHAIN_PARAMETERS.md](CHAIN_PARAMETERS.md) - mainnet, reboot testnet, seeds (including **`seed.dogego.org`**), genesis
- **First run:** [OPERATOR.md](OPERATOR.md) + setup wizard (`dogego node` without datadir)
- **Standalone mainnet (Beta):** [STANDALONE_NODE_QUICKSTART.md](STANDALONE_NODE_QUICKSTART.md) - build, sync, `dogego cert` (Windows / Linux / macOS)
- **Production mainnet / reboot testnet:** [CORE_OPERATOR_RUNBOOK.md](CORE_OPERATOR_RUNBOOK.md)
- **Dashboard:** [WEB_UI.md](WEB_UI.md) - default `http://localhost:2013/`
- **Mainnet + reboot testnet:** [MAINNET_TESTNET_OPERATIONAL.md](MAINNET_TESTNET_OPERATIONAL.md) - `dogego cert operational`, dual-run, founder checklist; testnet DNS **`seed.dogego.org`**
- **Scripts (all OS):** [scripts/README.md](../scripts/README.md) - prefer `dogego cert`; `.sh` / `.ps1` when needed
- **Connect your app:** [INTEGRATION.md](INTEGRATION.md)
- **RPC reference (live):** dashboard **Console** (cookbook + run) or **Features** tab; `GET /api/rpc/cookbook`
- **RPC tutorial:** [RPC_CONSOLE_TUTORIAL.md](RPC_CONSOLE_TUTORIAL.md) - Console, curl, shell examples, all methods
- **Protocol lock:** mainnet consensus follows Dogecoin Core; no protocol forks ([ROADMAP.md](../ROADMAP.md))
- **What we do not do:** [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md)
- **Core parity gaps:** [CORE_PARITY_GAPS.md](CORE_PARITY_GAPS.md)
- **Extensions & ZK L2:** [EXTENSIONS.md](EXTENSIONS.md), [extensions/catalog/zkl2/docs/USER_GUIDE.md](../extensions/catalog/zkl2/docs/USER_GUIDE.md), [extensions/catalog/README.md](../extensions/catalog/README.md)
- **Offline certification:** `go run ./cmd/dogego cert offline` (CI gate); `dogego cert wallet-import`; `dogego cert operator` (deep Milestone E); `dogego cert pq` (PQ format/carrier) - [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) § Offline certification

## For contributors

- **[CREDITS.md](CREDITS.md)** - acknowledgements (Raccoon-G author Ed Tubbs and others helping DogeGo)
- **[DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)** - repo layout, packages, tests, how to add RPC/consensus
- **[CHAIN_PARAMETERS.md](CHAIN_PARAMETERS.md)** - mainnet / reboot testnet / seeds / DNS / genesis (not one file; full map)

When you add or change behavior:

1. Update **`rpc/help.go`** one-line help for new RPCs.
2. Register the method in **`rpc/dispatch.go`** (`SupportedMethods`).
3. Add a **ROADMAP** checkbox when MVP-complete.
4. Add or extend a section in **RPC.md** / **WALLET.md** with an example `curl` or CLI call.
5. Extend **`ui/docs_index.go`** (served as `GET /api/docs`) so the **Docs** tab stays in sync.
