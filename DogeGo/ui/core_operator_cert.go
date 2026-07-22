// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dogego/config"
)

// CoreOperatorCertRow maps one scripts/core_operator_workflow_cert.ps1 live flag to a web probe or script-only gate.
type CoreOperatorCertRow struct {
	ID        string `json:"id"`
	EnvFlag   string `json:"env_flag,omitempty"`
	Label     string `json:"label"`
	Milestone string `json:"milestone"`
	WebProbe  bool   `json:"web_probe"`
	OK        *bool  `json:"ok,omitempty"`
	Script    string `json:"script,omitempty"`
	Note      string `json:"note,omitempty"`
}

// CoreOperatorCertResult is returned by GET /api/core-operator-cert.
type CoreOperatorCertResult struct {
	CheckedAt   string                `json:"checked_at"`
	OK          bool                  `json:"ok"`
	LiveOK      bool                  `json:"live_ok"`
	SoloOK      bool                  `json:"solo_ok,omitempty"`
	SoloPass    int                   `json:"solo_pass,omitempty"`
	Rows        []CoreOperatorCertRow `json:"rows"`
	Probes      CoreProbesBundle      `json:"probes,omitempty"`
	Hint        string                `json:"hint,omitempty"`
	MatrixOnly  bool                  `json:"matrix_only,omitempty"`
	Cached      bool                  `json:"cached,omitempty"`
	CacheAgeSec int                   `json:"cache_age_sec,omitempty"`
}

func boolPtr(v bool) *bool { return &v }

// DefaultCoreOperatorCertRows is the operator cert matrix (scripts/core_operator_workflow_cert.ps1).
func DefaultCoreOperatorCertRows() []CoreOperatorCertRow {
	return []CoreOperatorCertRow{
		{
			ID: "core_compare", EnvFlag: "DOGEGO_CORE_COMPARE", Label: "Core side-by-side compare",
			Milestone: "E", WebProbe: true, Script: "scripts/core_parity_probe.ps1",
		},
		{
			ID: "core_compare_with_core", Label: "Core compare one-shot (start + probe)",
			Milestone: "E", WebProbe: false, Script: "scripts/core_compare_with_core.ps1",
			Note: "Core :22555, DogeGo :22557 - optional -MempoolProbe",
		},
		{
			ID: "maintenance", EnvFlag: "DOGEGO_MAINTENANCE_PROBE", Label: "Maintenance RPC bundle",
			Milestone: "E", WebProbe: true, Script: "scripts/core_maintenance_workflow.ps1",
		},
		{
			ID: "restart_resume", EnvFlag: "DOGEGO_RESTART_RESUME", Label: "Restart resume checkpoint",
			Milestone: "E", WebProbe: true, Script: "scripts/core_restart_resume_check.ps1",
		},
		{
			ID: "cert_autostart", Label: "OS login autostart",
			Milestone: "E", WebProbe: true,
			Note: "Loopback probe + dogego cert autostart; restart-resume also checks os_autostart when autostart=login",
		},
		{
			ID: "cert_founder", Label: "Reboot testnet founder preflight",
			Milestone: "E", WebProbe: true,
			Note: "Loopback probe + dogego cert founder; skipped OK when network≠testnet",
		},
		{
			ID: "runner_readiness", Label: "CI runner readiness (dogego-live)",
			Milestone: "E", WebProbe: true,
			Note: "GET /api/core-runner-probes; dogego cert workflow10; workflow 10 runbook",
		},
		{
			ID: "restart_connect", EnvFlag: "DOGEGO_RESTART_CONNECT_CHECK", Label: "Connect lag catch-up",
			Milestone: "E", WebProbe: true, Script: "scripts/core_restart_connect_check.ps1",
			Note: "Snapshot only; script polls until lag clears",
		},
		{
			ID: "setup_reboottestnet_core_parity", Label: "Reboottestnet Core parity setup",
			Milestone: "D", WebProbe: true, Script: "scripts/setup_reboottestnet_core_parity.ps1",
			Note: "GET /api/core-setup-parity; preflight + optional -MineBootstrap before 24/24 gate; dogego cert setup-parity",
		},
		{
			ID: "mempool_parity", EnvFlag: "DOGEGO_MEMPOOL_PROBE", Label: "Mempool policy parity",
			Milestone: "D", WebProbe: true, Script: "scripts/core_mempool_parity_probe.ps1",
		},
		{
			ID: "reindex", EnvFlag: "DOGEGO_REINDEX_PROBE", Label: "Reindex / index check-only",
			Milestone: "E", WebProbe: true, Script: "scripts/core_reindex_prune_workflow.ps1",
		},
		{
			ID: "bip152_relay", EnvFlag: "DOGEGO_BIP152_PROBE", Label: "BIP152 compact block HB",
			Milestone: "E", WebProbe: true, Script: "scripts/core_bip152_probe.ps1",
			Note: "getpeerinfo bip152_hb_to/from; HB negotiate when caught up with peers",
		},
		{
			ID: "pq_format", Label: "Post-quantum format/carrier probe",
			Milestone: "E", WebProbe: true, Script: "scripts/pq_cert.ps1",
			Note: "GET /api/core-pq-probe; dogego cert pq offline gate; verifier-side only",
		},
		{
			ID: "wallet", Label: "Wallet basics",
			Milestone: "E", WebProbe: true, Script: "scripts/core_wallet_workflow.ps1",
		},
		{
			ID: "mining", EnvFlag: "DOGEGO_MINING_PROBE", Label: "Mining / GBT / aux templates",
			Milestone: "E", WebProbe: true, Script: "scripts/core_mining_workflow.ps1",
			Note: "GET /api/core-mining-probe; Digishield GBT bits + BIP22 longpoll + createauxblock in aux era; optional Core GBT compare",
		},
		{
			ID: "end_to_end", Label: "End-to-end workflow bundle",
			Milestone: "E", WebProbe: true, Script: "scripts/core_end_to_end_workflow.ps1",
			Note: "Live gate: GET /api/core-end-to-end-probe (incl. offline_corpus, bip125_offline, mempool_parity, mining); .ps1 optional Windows CI mirror",
		},
		{
			ID: "core_side_by_side_full", EnvFlag: "DOGEGO_CORE_COMPARE", Label: "Core side-by-side full bundle",
			Milestone: "E", WebProbe: false, Script: "scripts/core_side_by_side_full.ps1",
			Note: "Optional - parity + mempool + maintenance + restart-resume + end-to-end",
		},
		{
			ID: "operator_runbook_full", Label: "Full operator runbook (all flags)",
			Milestone: "E", WebProbe: false, Script: "scripts/core_operator_runbook_full.ps1",
			Note: "Bundles all DOGEGO_* cert flags; -OfflineOnly for CI",
		},
		{
			ID: "mainnet_side_by_side_runbook", Label: "Mainnet Core side-by-side runbook",
			Milestone: "E", WebProbe: false, Script: "scripts/core_mainnet_side_by_side_runbook.ps1",
			Note: "Non-disruptive: field evidence + mempool corpus + Core compare on :22555/:22557; requires -AllowMainnet",
		},
		{
			ID: "mempool_corpus_offline", Label: "Mempool corpus offline (58 templates)",
			Milestone: "D", WebProbe: false, Script: "scripts/core_mempool_corpus_probe.ps1",
		},
		{
			ID: "mempool_stateful_reboottestnet", EnvFlag: "DOGEGO_MEMPOOL_STATEFUL_PROBE", Label: "Stateful mempool probe (reboottestnet)",
			Milestone: "D", WebProbe: false, Script: "scripts/mempool_stateful_parity_reboottestnet.ps1",
			Note: "Wallet + mine; 24 live scenarios; Core compare when DOGEGO_CORE_COMPARE=1",
		},
		{
			ID: "mempool_stateful_core_gate", EnvFlag: "DOGEGO_MEMPOOL_STATEFUL_CORE_GATE", Label: "Stateful mempool Core gate",
			Milestone: "D", WebProbe: false, Script: "scripts/mempool_stateful_core_gate.ps1",
			Note: "24 scenarios with DOGEGO_CORE_COMPARE_REQUIRED=1; fails if Core unreachable",
		},
		{
			ID: "reboottestnet_core_aligned_gate", EnvFlag: "DOGEGO_REBOOTTESTNET_CORE_GATE", Label: "Reboottestnet Core-aligned gate",
			Milestone: "D", WebProbe: false, Script: "scripts/core_reboottestnet_core_aligned_gate.ps1",
			Note: "Core + DogeGo readiness then 24/24 stateful Core compare",
		},
		{
			ID: "mainnet_reindex_compare", Label: "Mainnet reindex/index compare vs Core",
			Milestone: "E", WebProbe: false, Script: "scripts/core_mainnet_reindex_compare.ps1",
		},
		{
			ID: "reboottestnet_reindex", EnvFlag: "DOGEGO_REBOOTTESTNET_REINDEX", Label: "Reboottestnet reindextx workflow",
			Milestone: "E", WebProbe: false, Script: "scripts/core_reboottestnet_reindex_workflow.ps1",
		},
		{
			ID: "reindex_disruptive", EnvFlag: "DOGEGO_REINDEX_DISRUPTIVE", Label: "Disruptive reindex/prune workflow",
			Milestone: "E", WebProbe: false, Script: "scripts/core_reindex_prune_disruptive_workflow.ps1",
			Note: "reindextx + optional filters/prune; mainnet needs -AllowMainnet -ConfirmDisruptive",
		},
		{
			ID: "reboottestnet_reindex_compare", Label: "Reboottestnet reindex compare vs Core",
			Milestone: "E", WebProbe: false, Script: "scripts/core_reboottestnet_reindex_compare.ps1",
		},
		{
			ID: "ci_scheduled_corruption_soak", Label: "CI scheduled corruption soak",
			Milestone: "B", WebProbe: false, Script: "scripts/ci_scheduled_corruption_soak.ps1",
			Note: "Offline gates + corruption_long_soak_gate; GHA live-soak job; cross-platform: dogego cert live-soak",
		},
		{
			ID: "ci_milestone_b_full_gate", Label: "Milestone B full corruption soak",
			Milestone: "B", WebProbe: false, Script: "scripts/ci_milestone_b_full_gate.ps1",
			Note: "Multi-hour reboottestnet soak + verifychain; dogego cert live-soak",
		},
		{
			ID: "runner_preflight", EnvFlag: "DOGEGO_RUNNER_PREFLIGHT", Label: "CI runner preflight",
			Milestone: "B", WebProbe: false, Script: "scripts/ci_runner_preflight.ps1",
			Note: "dogego-live readiness: dogego cert preflight (-require-core); workflow 10 runbook",
		},
		{
			ID: "scheduled_weekly_live", EnvFlag: "DOGEGO_WEEKLY_LIVE_GATE", Label: "Scheduled weekly live CI",
			Milestone: "E", WebProbe: false, Script: "scripts/ci_scheduled_weekly_live.ps1",
			Note: "Core 24/24 + corruption mini; cross-platform: dogego cert weekly-live (-skip-scripts smoke); workflow 10 runbook",
		},
		{
			ID: "gh_enable_scheduled_live", Label: "Enable scheduled live CI (gh)",
			Milestone: "E", WebProbe: false, Script: "scripts/gh_enable_scheduled_live.ps1",
			Note: "Sets DOGEGO_SCHEDULED_WEEKLY_LIVE repo variable via gh CLI; cross-platform: dogego cert enable-weekly",
		},
		{
			ID: "runner_provision_checklist", Label: "CI runner provision checklist",
			Milestone: "E", WebProbe: false, Script: "scripts/ci_runner_provision_checklist.ps1",
			Note: "Operator steps for dogego-live; dogego cert provision (-preflight -run-setup); CORE_SIDE_BY_SIDE_WORKFLOWS.md workflow 10",
		},
		{
			ID: "ci_live_reboottestnet", EnvFlag: "DOGEGO_CI_LIVE_GATE", Label: "CI live reboottestnet gate",
			Milestone: "E", WebProbe: false, Script: "scripts/ci_live_reboottestnet_gate.ps1",
			Note: "Health + E2E + Core-aligned 24/24 + corruption mini; GHA live-reboottestnet job",
		},
		{
			ID: "ci_offline_gate", Label: "CI offline gate",
			Milestone: "B", WebProbe: false, Script: "scripts/ci_offline_gate.ps1",
			Note: "go test bundle; also ci_offline_gate.sh on Linux GHA",
		},
		{
			ID: "mainnet_disruptive_reindex", Label: "Mainnet disruptive reindex gate",
			Milestone: "E", WebProbe: false, Script: "scripts/core_mainnet_disruptive_reindex_gate.ps1",
			Note: "Manual only: -AllowMainnet -ConfirmDisruptive",
		},
		{
			ID: "mainnet_restart_compare", Label: "Mainnet restart compare vs Core",
			Milestone: "E", WebProbe: false, Script: "scripts/core_mainnet_restart_compare.ps1",
			Note: "Disruptive DogeGo restart only; -AllowMainnet",
		},
		{
			ID: "mainnet_maintenance_compare", Label: "Mainnet maintenance compare vs Core",
			Milestone: "E", WebProbe: false, Script: "scripts/core_mainnet_maintenance_compare.ps1",
		},
		{
			ID: "operator_workflow_offline", Label: "Operator workflow (offline)",
			Milestone: "E", WebProbe: false, Script: "scripts/operator_workflow_cert.ps1",
			Note: "go test gates - no DOGEGO_* env required",
		},
		{
			ID: "ibd_soak", EnvFlag: "DOGEGO_IBD_SOAK", Label: "IBD soak gate",
			Milestone: "E", WebProbe: false, Script: "scripts/ibd_soak_cert.ps1",
			Note: "Timed live soak - run script",
		},
		{
			ID: "ibd_converge", EnvFlag: "DOGEGO_IBD_CONVERGE", Label: "IBD convergence snapshot",
			Milestone: "E", WebProbe: true, Script: "scripts/ibd_convergence_check.ps1",
			Note: "GET /api/core-ibd-convergence-probe (snapshot); script runs timed progress window",
		},
		{
			ID: "addrman", EnvFlag: "DOGEGO_ADDRMAN_PROBE", Label: "Addrman snapshot",
			Milestone: "E", WebProbe: true, Script: "scripts/core_addrman_workflow.ps1",
			Note: "GET /api/core-addrman-probe; skipped OK when P2P disabled",
		},
		{
			ID: "timed_soak", EnvFlag: "DOGEGO_TIMED_SOAK", Label: "Timed health soak",
			Milestone: "E", WebProbe: false, Script: "scripts/ibd_timed_soak.ps1",
		},
		{
			ID: "ibd_live_soak", EnvFlag: "DOGEGO_IBD_LIVE_SOAK", Label: "Mainnet IBD live gate",
			Milestone: "E", WebProbe: false, Script: "scripts/ibd_live_soak_gate.ps1",
		},
		{
			ID: "extended_soak", EnvFlag: "DOGEGO_EXTENDED_SOAK", Label: "Extended operator soak",
			Milestone: "E", WebProbe: false, Script: "scripts/extended_operator_soak.ps1",
		},
		{
			ID: "corruption_soak", EnvFlag: "DOGEGO_CORRUPTION_SOAK", Label: "Corruption / kill recovery",
			Milestone: "B", WebProbe: false, Script: "scripts/corruption_soak_cert.ps1",
			Note: "Offline go test + subprocess kill",
		},
		{
			ID: "restart_workflow", EnvFlag: "DOGEGO_RESTART_WORKFLOW", Label: "Disruptive restart workflow",
			Milestone: "E", WebProbe: false, Script: "scripts/core_restart_workflow.ps1",
			Note: "Stop/start node - disruptive",
		},
		{
			ID: "corruption_inject", EnvFlag: "DOGEGO_CORRUPTION_INJECT", Label: "Live corruption inject",
			Milestone: "B", WebProbe: false, Script: "scripts/corruption_inject_live.ps1",
			Note: "Disruptive - testnet/dev only",
		},
		{
			ID: "corruption_inject_soak", EnvFlag: "DOGEGO_CORRUPTION_INJECT_SOAK", Label: "Corruption inject soak",
			Milestone: "B", WebProbe: false, Script: "scripts/corruption_inject_soak.ps1",
			Note: "Disruptive - testnet/dev only",
		},
		{
			ID: "corruption_timed_loop", EnvFlag: "DOGEGO_CORRUPTION_TIMED_LOOP", Label: "Timed corruption loop",
			Milestone: "B", WebProbe: false, Script: "scripts/corruption_timed_loop.ps1",
			Note: "Disruptive; use corruption_timed_loop_mini.ps1 for short cert",
		},
		{
			ID: "corruption_extended_mini", EnvFlag: "DOGEGO_CORRUPTION_EXTENDED_MINI", Label: "Extended corruption cert mini",
			Milestone: "B", WebProbe: false, Script: "scripts/corruption_extended_cert_mini.ps1",
			Note: "Health soak + timed loop on headers/raw/filter/txindex",
		},
		{
			ID: "corruption_long_soak", EnvFlag: "DOGEGO_CORRUPTION_LONG_SOAK", Label: "Corruption long soak gate",
			Milestone: "B", WebProbe: false, Script: "scripts/corruption_long_soak_gate.ps1",
			Note: "Disruptive; default 45m (DOGEGO_CORRUPTION_LONG_MIN); health + timed loop on all targets",
		},
		{
			ID: "recovery_workflow", EnvFlag: "DOGEGO_RECOVERY_PROBE", Label: "Recovery workflow probe",
			Milestone: "E", WebProbe: false, Script: "scripts/core_recovery_workflow.ps1",
			Note: "dogego_recoverheaders RPC presence; -InvokeRecover disruptive",
		},
		{
			ID: "e2e_reboottestnet_runbook", Label: "Reboottestnet E2E runbook",
			Milestone: "E", WebProbe: false, Script: "scripts/core_e2e_reboottestnet_runbook.ps1",
			Note: "Health + maintenance + wallet + stateful mempool (-Scenario all); -IncludeCoreCompare",
		},
		{
			ID: "e2e_full_runbook", Label: "Reboottestnet full E2E runbook",
			Milestone: "E", WebProbe: false, Script: "scripts/core_e2e_full_runbook.ps1",
			Note: "Offline cert + IBD convergence + reindex/prune + recovery + wallet + stateful; -IncludeDisruptive",
		},
		{
			ID: "e2e_mainnet_runbook", Label: "Mainnet read-only E2E runbook",
			Milestone: "E", WebProbe: false, Script: "scripts/core_e2e_mainnet_runbook.ps1",
			Note: "Non-disruptive: corpus + side-by-side + maintenance + reindex compare (-AllowMainnet)",
		},
	}
}

func cloneOperatorCertRows(in []CoreOperatorCertRow) []CoreOperatorCertRow {
	out := make([]CoreOperatorCertRow, len(in))
	copy(out, in)
	return out
}

// ApplyCoreOperatorCertProbes fills web-gate OK fields from a probe bundle.
func ApplyCoreOperatorCertProbes(rows []CoreOperatorCertRow, probes CoreProbesBundle) []CoreOperatorCertRow {
	rows = cloneOperatorCertRows(rows)
	for i := range rows {
		switch rows[i].ID {
		case "core_compare":
			if !probes.Compare.Available {
				if probes.Compare.CoreConfigured {
					rows[i].Note = "Core not reachable - configure core_rpc_addr"
				} else if probes.Compare.DeploymentChecked {
					rows[i].OK = boolPtr(probes.Compare.ProtocolLockOK)
					if probes.Compare.ProtocolLockOK {
						rows[i].Note = "Solo protocol-lock sanity OK (Core compare optional)"
					} else {
						rows[i].Note = "Deployment active-state mismatch vs consensus params"
					}
					continue
				} else {
					rows[i].OK = boolPtr(true)
					rows[i].Note = "Core compare optional - set core_rpc_addr for side-by-side cert"
					continue
				}
			}
			ok := probes.Compare.ChainOK && probes.Compare.VerifyOK
			if probes.Compare.DeploymentChecked && !probes.Compare.ProtocolLockOK {
				ok = false
			}
			rows[i].OK = boolPtr(ok)
		case "maintenance":
			rows[i].OK = boolPtr(probes.Maintenance.OK)
		case "restart_resume":
			rows[i].OK = boolPtr(probes.RestartResume.OK)
		case "cert_autostart":
			as := probes.Autostart
			rows[i].OK = boolPtr(as.OK)
			if len(as.Issues) > 0 {
				rows[i].Note = as.Issues[0]
			} else if len(as.Warnings) > 0 {
				rows[i].Note = as.Warnings[0]
			} else if !as.WantLogin {
				rows[i].Note = "autostart=disable - skipped"
			}
		case "cert_founder":
			fp := probes.Founder
			if fp.Skipped {
				rows[i].OK = boolPtr(true)
				rows[i].Note = fp.SkipReason
			} else {
				rows[i].OK = boolPtr(fp.OK)
				if !fp.OK && len(fp.Verify.Issues) > 0 {
					rows[i].Note = fp.Verify.Issues[0]
				} else if len(fp.Verify.Warnings) > 0 {
					rows[i].Note = fp.Verify.Warnings[0]
				}
			}
		case "runner_readiness":
			rr := probes.Runner
			if rr.Skipped {
				rows[i].OK = boolPtr(true)
				rows[i].Note = rr.SkipReason
			} else {
				rows[i].OK = boolPtr(rr.OK)
				if !rr.OK {
					if len(rr.Preflight.Issues) > 0 {
						rows[i].Note = rr.Preflight.Issues[0]
					} else if len(rr.Provision.Issues) > 0 {
						rows[i].Note = rr.Provision.Issues[0]
					}
				} else if rr.Doc != "" {
					rows[i].Note = rr.Doc
				}
			}
		case "restart_connect":
			rr := probes.RestartResume
			ok := true
			if !rr.IBD {
				for _, iss := range rr.Issues {
					if iss == "connect_lag_above_threshold" {
						ok = false
					}
				}
				maxLag := connectLagMax()
				if rr.ConnectLag > maxLag && rr.ConnectCatchUpPasses <= 0 {
					ok = false
				}
			}
			var noteParts []string
			if rr.IBD && rr.ConnectLag > connectLagMax() {
				noteParts = append(noteParts, "high connect lag expected during IBD")
			}
			if boostNote := restartResumeE2ENote(rr); boostNote != "" {
				noteParts = append(noteParts, boostNote)
			}
			rows[i].Note = strings.Join(noteParts, " · ")
			rows[i].OK = boolPtr(ok)
		case "setup_reboottestnet_core_parity":
			sp := probes.SetupParity
			if sp.Skipped {
				rows[i].OK = boolPtr(true)
				rows[i].Note = sp.SkipReason
			} else {
				rows[i].OK = boolPtr(sp.OK)
				if sp.Setup.DogeGoBalance > 0 {
					rows[i].Note = fmt.Sprintf("dogego_balance=%g", sp.Setup.DogeGoBalance)
				}
				if !sp.OK && len(sp.Setup.Issues) > 0 {
					rows[i].Note = sp.Setup.Issues[0]
				} else if sp.CLI != "" && rows[i].Note == "" {
					rows[i].Note = sp.CLI
				}
			}
		case "mempool_parity":
			mp := probes.MempoolParity
			if mp.Skipped {
				rows[i].OK = boolPtr(mempoolParitySkipOK(mp.Reason))
				rows[i].Note = mp.Reason
				if rows[i].Note == "" {
					rows[i].Note = "skipped"
				}
			} else {
				ok := mp.OK
				if mp.CoreConfigured && mp.CoreAvailable && !mp.CoreAligned {
					ok = false
					rows[i].Note = "Core drift on one or more rows"
				} else if !mp.CoreConfigured && mp.OK {
					rows[i].Note = "Core compare optional - set core_rpc_addr for side-by-side cert"
					if mp.OfflineCorpus != nil && mp.OfflineCorpus.Total > 0 {
						rows[i].Note += fmt.Sprintf("; offline corpus %d/%d", mp.OfflineCorpus.Passed, mp.OfflineCorpus.Total)
					}
					if mp.OfflineStateful != nil && mp.OfflineStateful.Total > 0 {
						rows[i].Note += fmt.Sprintf("; offline stateful %d/%d", mp.OfflineStateful.Passed, mp.OfflineStateful.Total)
					}
					if mp.StatefulLive != nil && mp.StatefulLive.RebootTestnet && mp.StatefulLive.OfflineOK {
						rows[i].Note += fmt.Sprintf("; live %d scenarios via %s", mp.StatefulLive.LiveScenarios, mp.StatefulLive.ScriptLive)
						if mp.StatefulLive.SetupParityProbe != "" {
							if mp.StatefulLive.SetupParityOK {
								rows[i].Note += "; setup_parity ok"
							} else if !mp.StatefulLive.SetupParitySkipped {
								rows[i].Note += "; setup_parity pending (" + mp.StatefulLive.SetupParityCLI + ")"
							}
						}
					}
				} else if mp.OK {
					rows[i].Note = fmt.Sprintf("stateless %d/%d", mp.Passed, mp.Total)
					if mp.OfflineCorpus != nil && mp.OfflineCorpus.Total > 0 {
						rows[i].Note += fmt.Sprintf("; offline corpus %d/%d", mp.OfflineCorpus.Passed, mp.OfflineCorpus.Total)
					}
				}
				rows[i].OK = boolPtr(ok)
			}
		case "mempool_corpus_offline":
			if oc := probes.MempoolParity.OfflineCorpus; oc != nil && oc.Total > 0 {
				rows[i].OK = boolPtr(oc.OK)
				rows[i].Note = fmt.Sprintf("offline %d/%d (incl. BIP125 rule 2/5)", oc.Passed, oc.Total)
			}
		case "reindex":
			rows[i].OK = boolPtr(probes.Reindex.OK)
		case "bip152_relay":
			bp := probes.Bip152
			if bp.Skipped {
				rows[i].OK = boolPtr(true)
				if bp.Reason != "" {
					rows[i].Note = bp.Reason
				} else {
					rows[i].Note = "skipped"
				}
			} else {
				rows[i].OK = boolPtr(bp.OK)
				var noteParts []string
				if bp.HBToPeers > 0 || bp.HBFromPeers > 0 {
					noteParts = append(noteParts, fmt.Sprintf("hb_to=%d hb_from=%d", bp.HBToPeers, bp.HBFromPeers))
				}
				if bp.CmpctRelaySchemaOK {
					noteParts = append(noteParts, "cmpct_schema ok")
				} else if bp.PeerCount > 0 && !bp.IBD {
					noteParts = append(noteParts, "cmpct_schema pending")
				}
				if len(bp.Notes) > 0 {
					noteParts = append(noteParts, bp.Notes[0])
				} else if len(bp.Issues) > 0 {
					noteParts = append(noteParts, bp.Issues[0])
				}
				rows[i].Note = strings.Join(noteParts, " · ")
			}
		case "pq_format":
			pq := probes.PQ
			rows[i].OK = boolPtr(pq.OK)
			if len(pq.Issues) > 0 {
				rows[i].Note = pq.Issues[0]
			} else if len(pq.Checks) > 0 {
				rows[i].Note = fmt.Sprintf("%d checks ok (offline format/carrier)", len(pq.Checks))
			}
		case "wallet":
			if probes.Wallet.Skipped {
				rows[i].OK = boolPtr(true)
				rows[i].Note = "wallet not enabled - skipped"
			} else {
				rows[i].OK = boolPtr(probes.Wallet.OK)
				if probes.Wallet.OK && len(probes.Wallet.Warnings) == 0 && len(probes.Wallet.Notes) > 0 {
					rows[i].Note = probes.Wallet.Notes[0]
				} else if !probes.Wallet.OK && len(probes.Wallet.Issues) > 0 {
					rows[i].Note = probes.Wallet.Issues[0]
				}
			}
		case "mining":
			mp := probes.Mining
			if mp.CheckedAt == "" && !mp.OK && len(mp.Issues) == 0 && len(mp.Checks) == 0 {
				rows[i].OK = boolPtr(true)
				rows[i].Note = "mining probe not bundled"
			} else {
				rows[i].OK = boolPtr(mp.OK)
				var noteParts []string
				if mp.GBTFieldsOK {
					noteParts = append(noteParts, "gbt ok")
				}
				if mp.CreateAuxOK {
					noteParts = append(noteParts, "createaux ok")
				} else if mp.CreateAuxSkipped {
					noteParts = append(noteParts, "createaux skipped (pre-aux)")
				}
				if mp.CoreAligned {
					noteParts = append(noteParts, "core_gbt_aligned")
				} else if mp.CoreConfigured && !mp.CoreAvailable {
					noteParts = append(noteParts, "Core optional for GBT compare")
				}
				if !mp.OK && len(mp.Issues) > 0 {
					noteParts = []string{mp.Issues[0]}
				} else if len(mp.Notes) > 0 && len(noteParts) == 0 {
					noteParts = append(noteParts, mp.Notes[0])
				}
				rows[i].Note = strings.Join(noteParts, " · ")
			}
		case "ibd_converge":
			ic := probes.IbdConvergence
			if ic.Skipped {
				rows[i].OK = boolPtr(true)
				rows[i].Note = ic.Reason
			} else {
				rows[i].OK = boolPtr(ic.OK)
				if ic.ConnectBoost != "" {
					rows[i].Note = "connect_boost=" + ic.ConnectBoost
				}
				if ic.BodyCoveragePct > 0 {
					if rows[i].Note != "" {
						rows[i].Note += " · "
					}
					rows[i].Note += fmt.Sprintf("body_coverage=%.1f%%", ic.BodyCoveragePct)
				}
				if !ic.OK && len(ic.Issues) > 0 {
					rows[i].Note = ic.Issues[0]
				} else if rows[i].Note == "" && len(ic.Notes) > 0 {
					rows[i].Note = ic.Notes[0]
				}
			}
		case "addrman":
			am := probes.Addrman
			if am.Skipped {
				rows[i].OK = boolPtr(true)
				if am.Reason == "p2p_disabled" {
					rows[i].Note = "p2p disabled - skipped"
				} else {
					rows[i].Note = am.Reason
				}
			} else {
				rows[i].OK = boolPtr(am.OK)
				if am.Tried != nil && am.NewAddrs != nil {
					rows[i].Note = fmt.Sprintf("tried=%d new=%d", *am.Tried, *am.NewAddrs)
				}
				if am.NKeySet != nil && *am.NKeySet {
					if rows[i].Note != "" {
						rows[i].Note += " · "
					}
					rows[i].Note += "n_key set"
				}
				if !am.OK && len(am.Issues) > 0 {
					rows[i].Note = am.Issues[0]
				} else if rows[i].Note == "" && len(am.Notes) > 0 {
					rows[i].Note = am.Notes[0]
				}
			}
		case "end_to_end":
			e2e := probes.EndToEnd
			if e2e.CheckedAt == "" {
				e2e = EndToEndFromProbes(probes)
			}
			rows[i].OK = boolPtr(e2e.OK)
			if !e2e.OK {
				for _, step := range e2e.Steps {
					if step.Skipped || step.OK {
						continue
					}
					rows[i].Note = "failed: " + step.Name
					break
				}
			}
		}
	}
	return rows
}

func operatorCertLiveOK(rows []CoreOperatorCertRow) bool {
	for _, r := range rows {
		if !r.WebProbe || r.OK == nil {
			continue
		}
		if !*r.OK {
			return false
		}
	}
	return true
}

// operatorCertSoloMetrics counts web gates that pass on a solo node (optional Core / skipped rows count as pass).
func operatorCertSoloMetrics(rows []CoreOperatorCertRow) (pass, total int, ok bool) {
	for _, r := range rows {
		if !r.WebProbe {
			continue
		}
		total++
		if certRowSoloPass(r) {
			pass++
		}
	}
	ok = total > 0 && pass == total
	return pass, total, ok
}

func certRowSoloPass(r CoreOperatorCertRow) bool {
	if r.OK != nil && *r.OK {
		return true
	}
	n := strings.ToLower(strings.TrimSpace(r.Note))
	if n == "" {
		return false
	}
	return strings.Contains(n, "optional") ||
		strings.Contains(n, "skipped") ||
		strings.Contains(n, "rpc not ready") ||
		strings.Contains(n, "warming up") ||
		strings.Contains(n, "hb may be deferred") ||
		strings.Contains(n, "hb_not_negotiated") ||
		strings.Contains(n, "high connect lag expected") ||
		strings.Contains(n, "connect catch-up") ||
		strings.Contains(n, "autostart=disable") ||
		strings.Contains(n, "not reboot testnet")
}

func mempoolParitySkipOK(reason string) bool {
	r := strings.ToLower(strings.TrimSpace(reason))
	if r == "" {
		return true
	}
	return strings.Contains(r, "rpc not ready") ||
		strings.Contains(r, "warming up") ||
		strings.Contains(r, "rpc_in_warmup")
}

// CoreOperatorCertMatrix returns the cert matrix without running probes (optional cached OK merge).
func CoreOperatorCertMatrix(cached *CoreOperatorCertResult) CoreOperatorCertResult {
	rows := DefaultCoreOperatorCertRows()
	checkedAt := time.Now().UTC().Format(time.RFC3339)
	liveOK := false
	cacheAge := 0
	cachedFlag := false
	if cached != nil && cached.CheckedAt != "" {
		rows = cloneOperatorCertRows(cached.Rows)
		checkedAt = cached.CheckedAt
		liveOK = cached.LiveOK
		cacheAge = cached.CacheAgeSec
		cachedFlag = cached.Cached
	}
	return CoreOperatorCertResult{
		CheckedAt:   checkedAt,
		OK:          liveOK,
		LiveOK:      liveOK,
		Rows:        rows,
		MatrixOnly:  true,
		Cached:      cachedFlag,
		CacheAgeSec: cacheAge,
		Hint:        "Certification matrix. Live rows use built-in HTTP probes (no PowerShell). Script-only rows are optional Windows CI helpers.",
	}
}

// RunCoreOperatorCert aggregates live web probes and documents script-only certification gates.
func RunCoreOperatorCert(network, dogeRPCAddr, chainDataDir string, conf config.File, invoke func(string, []json.RawMessage) map[string]interface{}) CoreOperatorCertResult {
	probes := RunCoreProbes(network, dogeRPCAddr, chainDataDir, conf, invoke)
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), probes)
	liveOK := operatorCertLiveOK(rows)
	soloPass, _, soloOK := operatorCertSoloMetrics(rows)
	return CoreOperatorCertResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		OK:        liveOK,
		LiveOK:    liveOK,
		SoloOK:    soloOK,
		SoloPass:  soloPass,
		Rows:      rows,
		Probes:    probes,
		Hint:      "Live web gates run inside DogeGo (loopback HTTP). Script-only rows list optional scripts/*.ps1 for Windows operators with dogecoin-cli - not required for normal use.",
	}
}
