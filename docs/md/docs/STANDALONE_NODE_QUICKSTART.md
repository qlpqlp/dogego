# DogeGo standalone full node - quick start

Beta Go full node. Not Dogecoin Core. Use a **dedicated datadir**; storage is not interchangeable with Core `blocks/` + `chainstate/`.

Works on **Windows, Linux, and macOS**. Prefer `dogego cert …` and the web dashboard over OS-specific scripts. Script twins: see [scripts/README.md](../scripts/README.md).

## 1. Build

```bash
cd DogeGo
go build -o dogego ./cmd/dogego
```

Windows tip: you can name the binary `dogego.exe` (`go build -o dogego.exe ./cmd/dogego`).

## 2. First run (setup wizard)

```bash
./dogego
```

Open **http://localhost:2013/** - complete setup (network **mainnet** for public chain, **testnet** for reboot testnet). Recommended for a public full node: `p2p_connectivity=both`, `upnp=auto`, bundled+zstd block storage.

**Reboot testnet:** peer discovery queries **`seed.dogego.org` first** (DogeBox running a DogeGo full node on the new testnet, so fresh installs find a peer quickly), then Core fixed seeds. Details: [CHAIN_PARAMETERS.md](CHAIN_PARAMETERS.md).

## 3. Verify build capability (mainnet)

With the node running, prefer the dashboard **Features** probes, or:

```bash
# Any OS (HTTP loopback; node must be up)
curl -sS "http://localhost:2013/api/core-mining-probe" | head
```

Expect auxpow / mining probe fields healthy. Windows operators can also run `scripts/upgrade_post_aux_verify.ps1`.

## 4. Watch sync

| Goal | Cross-platform | Windows helper (optional) |
|------|----------------|---------------------------|
| Status | Overview → Sync, or `getblockchaininfo` via Console / curl | `scripts/sync_status.ps1` |
| Live watch | Overview sync dock | `scripts/watch_sync.ps1` |
| Resume after stop | start `./dogego` again (same datadir) | `scripts/resume_node.ps1` |
| Header past 510k | `headers_sync.json` tip, or Console `getblockchaininfo` | `scripts/check_header_progress.ps1` |

Example curl:

```bash
curl -sS --user USER:PASS -H 'content-type: application/json' \
  -d '{"jsonrpc":"1.0","id":1,"method":"getblockchaininfo","params":[]}' \
  http://127.0.0.1:22557/
```

During IBD, **header %** can sit near **~8%** while **blocks** / stored bodies are much lower - normal. Headers should pass **510000** within minutes on a current build (post-aux era). Confirm on disk: `dogedata/mainnet/headers_sync.json` `tip_height`.

Optional long-haul logging (Windows): `scripts/log_ibd_progress.ps1 -DiskOnly -OutFile body_ibd.csv -IntervalSec 120`. Cross-platform soak gate: `dogego cert ibd-convergence`.

## 5. Offline certification (developers)

Same commands on Windows, Linux, and macOS:

```bash
go run ./cmd/dogego cert offline
go run ./cmd/dogego cert wallet-import
go run ./cmd/dogego cert pq          # optional PQ format/carrier (~40s)
go run ./cmd/dogego cert operator   # optional deep Milestone E (~5-20 min)
```

Optional script wrappers (same gates):

| Cert | Linux / macOS | Windows |
|------|---------------|---------|
| Offline bundle | `./scripts/cert_offline_prerequisites.sh` then `./scripts/ci_offline_gate.sh` | `.\scripts\cert_offline_prerequisites.ps1` / `ci_offline_gate.ps1` |
| Operator | `./scripts/operator_workflow_cert.sh` | `.\scripts\operator_workflow_cert.ps1` |
| PQ / wallet import | `./scripts/pq_cert.sh`, `wallet_import_cert.sh` | `.\scripts\pq_cert.ps1`, `wallet_import_cert.ps1` |

## 6. Deeper docs

| Doc | Topic |
|-----|--------|
| [scripts/README.md](../scripts/README.md) | Which scripts are cross-platform vs Windows helpers |
| [CORE_OPERATOR_RUNBOOK.md](CORE_OPERATOR_RUNBOOK.md) | IBD stalls, recovery, mining |
| [CORE_SIDE_BY_SIDE_WORKFLOWS.md](CORE_SIDE_BY_SIDE_WORKFLOWS.md) | Compare with Core |
| [STANDALONE_FULLNODE_ACCEPTANCE.md](STANDALONE_FULLNODE_ACCEPTANCE.md) | Release gate matrix |
| [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md) | What differs from Core |
| [OPERATOR.md](OPERATOR.md) | Datadir, RPC, sync knobs |
| [MAINNET_TESTNET_OPERATIONAL.md](MAINNET_TESTNET_OPERATIONAL.md) | Dual-run, founder checklist |
