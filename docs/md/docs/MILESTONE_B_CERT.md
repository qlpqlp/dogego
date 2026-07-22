# Milestone B certification (crash/corruption + IBD soak)

Milestone B proves **kill-and-restart convergence** and **timed corruption recovery** without manual repair. DogeGo is **partial** until multi-hour live soak runs green on a provisioned `dogego-live` runner.

## Tiers

| Tier | Network | Duration | Entry |
|------|---------|----------|--------|
| **offline** | none | ~5 min | `.\scripts\milestone_b_cert.ps1` |
| **mini** | reboottestnet | ~15-25 min | `.\scripts\milestone_b_cert.ps1 -Tier mini` |
| **live** | reboottestnet | ~30-60 min | `.\scripts\milestone_b_cert.ps1 -Tier live` |
| **long** | reboottestnet | 45+ min | `.\scripts\milestone_b_cert.ps1 -Tier long` |
| **extended** | reboottestnet | 20+ min | `.\scripts\milestone_b_cert.ps1 -Tier extended` |
| **mainnet-ibd** | mainnet | 20+ min | `.\scripts\milestone_b_cert.ps1 -Tier mainnet-ibd` |
| **full** | reboottestnet | 45+ min | `.\scripts\milestone_b_cert.ps1 -Tier full` |

**offline** runs `corruption_soak_cert.ps1` (subprocess kill + startup recovery tests). For the full offline CI gate including consensus corpus and wallet import fixtures, run `go run ./cmd/dogego cert offline` and `go run ./cmd/dogego cert wallet-import` (or `.\scripts\ci_offline_gate.ps1` separately; Milestone E).

**mini** runs `corruption_extended_cert_mini.ps1` (timed IBD health + short corruption loop on headers/raw/filter/txindex).

**full** is the Milestone B exit candidate: preflight + `corruption_long_soak_gate.ps1` + `verifychain 4 0`.

**mainnet-ibd** validates forward body IBD on your live mainnet node (`ibd_live_soak_gate.ps1`); use during multi-day mainnet sync, not on reboottestnet.

## Enable scheduled CI (dogego-live runner)

1. Provision runner - see `dogego cert provision -preflight -run-setup` or `.\scripts\ci_runner_provision_checklist.ps1 -RunPreflight -RunSetup`
2. Enable repo variables:
   ```powershell
   go run ./cmd/dogego cert enable-weekly -require-wallet-dat
   # or: .\scripts\gh_enable_scheduled_live.ps1 -RequireWalletDat
   ```
   Sets `DOGEGO_SCHEDULED_WEEKLY_LIVE`, `DOGEGO_SCHEDULED_CORE_GATE`, `DOGEGO_SCHEDULED_LIVE_SOAK`.
3. Cross-platform bundles (dogego-live):
   ```powershell
   # smoke (preflight only, no PS1 Core 24/24 gate):
   go run ./cmd/dogego cert weekly-live -skip-scripts -mine-bootstrap -require-wallet-dat
   # full weekly bundle:
   go run ./cmd/dogego cert weekly-live -mine-bootstrap -require-wallet-dat
   go run ./cmd/dogego cert live-soak -duration-min 60 -require-soak-env
   ```
   See [CORE_SIDE_BY_SIDE_WORKFLOWS.md](CORE_SIDE_BY_SIDE_WORKFLOWS.md) workflow 10.
4. Manual dispatch (GitHub Actions → DogeGo workflow):
   - `live_soak=true` - Milestone B full corruption soak
   - `live_weekly=true` - Core 24/24 + corruption mini
   - `live_e2e=true` - full reboottestnet gate

Weekly schedule (Sunday 04:00 UTC) runs jobs when the matching repo variable is `1`.
Prepared runners that keep a Core-generated fixture can also set `DOGEGO_WALLET_DAT`, optional `DOGEGO_WALLET_DAT_PASSPHRASE`, and `DOGEGO_WALLET_DAT_REQUIRED=1`; then run `dogego cert weekly -require-wallet-dat` or `.\scripts\ci_scheduled_weekly_live.ps1 -RequireWalletDat` to make live wallet.dat RPC import part of weekly readiness (native import may return **`pool_indices_replayed`** when matched HD receive pubkeys replay via `wallet/pool_replay.go`; pool-only rows return **`pool_unmatched_hint`** and **`keypool_refill_size`**). Use `.\scripts\provision_wallet_dat_fixture.ps1` to locate a Core `wallet.dat`, set user env vars, and run a live probe.

## Environment flags (core_operator_workflow_cert.ps1)

| Flag | Effect |
|------|--------|
| `DOGEGO_CORRUPTION_SOAK=1` | Offline corruption cert |
| `DOGEGO_TIMED_SOAK=1` | `ibd_timed_soak.ps1` |
| `DOGEGO_IBD_LIVE_SOAK=1` | Mainnet IBD live soak gate |
| `DOGEGO_EXTENDED_SOAK=1` | Timed IBD + corruption inject |
| `DOGEGO_CORRUPTION_LONG_SOAK=1` | 45+ min corruption long soak |
| `DOGEGO_MILESTONE_B_TIER=full` | Route via `milestone_b_cert.ps1` |

## What closes Milestone B (full)

- [ ] `DOGEGO_SCHEDULED_LIVE_SOAK=1` on self-hosted `dogego-live` runner
- [ ] Weekly `live-soak` job green for several consecutive runs
- [ ] `verifychain 4 0` passes after each long soak
- [ ] No manual datadir repair between inject cycles

Mainnet body IBD soak (`mainnet-ibd` tier) is **operator field evidence** complementary to reboottestnet corruption cert.
