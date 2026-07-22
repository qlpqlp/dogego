# DogeGo scripts

Operator and CI helpers live here. **You do not need these scripts to run a node** (use the binary + web UI). Prefer the cross-platform Go CLI when something has a `dogego cert` equivalent.

## Prefer this on every OS

From the `DogeGo/` directory:

```bash
go run ./cmd/dogego cert offline
go run ./cmd/dogego cert mining
go run ./cmd/dogego cert pq
go run ./cmd/dogego cert operator
go run ./cmd/dogego cert --help
```

Live probes (no PowerShell): Features tab, or `GET /api/core-probes` / `GET /api/core-operator-cert` while the node is running.

Details: [docs/DEVELOPER_GUIDE.md](../docs/DEVELOPER_GUIDE.md) § Offline prerequisites / certification.

## Platform notes

| Kind | When to use |
|------|-------------|
| **`dogego cert …`** | Default on Windows, Linux, and macOS |
| **`*.sh`** | Linux/macOS shell when a twin exists |
| **`*.ps1`** | Windows PowerShell, or `pwsh` on Linux/macOS; also used by `dogego-live` / GitHub Actions |

Keep the `.ps1` files in git. They are part of CI and Windows operator runbooks, not disposable Windows-only leftovers.

## Cert / gate map (`dogego cert` ↔ scripts)

| Prefer | PowerShell | Shell (Linux/macOS) |
|--------|------------|---------------------|
| `dogego cert offline` | `ci_offline_gate.ps1` | `ci_offline_gate.sh` |
| `dogego cert field-evidence` | `field_evidence_cert.ps1` | `field_evidence_cert.sh` |
| `dogego cert wallet-import` | `wallet_import_cert.ps1` | `wallet_import_cert.sh` |
| `dogego cert wallet-migration` | `wallet_migration_cert.ps1` | `wallet_migration_cert.sh` |
| `dogego cert operator` | `operator_workflow_cert.ps1` | `operator_workflow_cert.sh` |
| `dogego cert pq` | `pq_cert.ps1` | `pq_cert.sh` |
| `dogego cert mining` | `core_mining_workflow.ps1` | (use `dogego cert mining`) |
| `dogego cert bip152-soak` | `bip152_live_soak_gate.ps1` / `bip152_timed_soak.ps1` | (offline via cert; live soak often PS1) |
| `dogego cert ibd-convergence` | `ibd_convergence_check.ps1` | (use cert) |
| `dogego cert setup-parity` | `setup_reboottestnet_core_parity.ps1` | (use cert) |
| `dogego cert preflight` / `provision` / `weekly` / `weekly-live` | `ci_runner_preflight.ps1`, `ci_scheduled_weekly_live.ps1`, … | (use cert) |
| `dogego cert live-soak` | `ci_milestone_b_full_gate.ps1` | (use cert) |
| `dogego cert workflow10` | orchestrates provision → weekly-live (+ optional soak) | (use cert) |
| `dogego cert enable-weekly` | `gh_enable_scheduled_live.ps1` | (use cert; needs `gh`) |
| `dogego cert autostart` / `founder` / `operational` | related operator PS1 probes | (use cert) |
| `dogego cert milestones-bde` | milestone B/D/E offline bundle | (use cert) |

Prerequisite helpers: `cert_offline_prerequisites.ps1` / `cert_offline_prerequisites.sh`.

## Other script groups (no full `dogego cert` twin yet)

These stay as PowerShell (or occasional `.sh`) for Windows operators and CI:

- **Node health / IBD:** `node_health.ps1`, `sync_status.ps1`, `watch_sync.ps1`, `ibd_*.ps1`, `log_ibd_progress.ps1`, …
- **Core side-by-side / runbooks:** `core_*_workflow.ps1`, `core_*_runbook.ps1`, `core_compare_with_core.ps1`, …
- **Corruption / soak:** `corruption_*.ps1`, `extended_operator_soak.ps1`, `ibd_live_soak_gate.ps1`, …
- **Updates:** `check_update.ps1` / `check_update.sh`, `schedule_update_check.ps1` / `.sh`
- **Misc:** `lan_peer_pair.ps1`, `dogego_rpc.ps1`, `apply_mit_license.ps1`, field export/verify scripts

Web UI Features cards often mirror the same workflows without opening a terminal.

## Quick start

```bash
# Any OS - offline cert (no node required)
cd DogeGo
go run ./cmd/dogego cert offline

# Linux/macOS shell twin of the offline gate
./scripts/ci_offline_gate.sh

# Windows
.\scripts\ci_offline_gate.ps1
```
