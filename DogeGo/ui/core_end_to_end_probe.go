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

// CoreEndToEndStep is one step in the in-process end-to-end operator probe.
type CoreEndToEndStep struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Skipped bool   `json:"skipped,omitempty"`
	Note    string `json:"note,omitempty"`
}

// CoreEndToEndProbeResult is returned by GET /api/core-end-to-end-probe.
type CoreEndToEndProbeResult struct {
	CheckedAt string             `json:"checked_at"`
	OK        bool               `json:"ok"`
	Steps     []CoreEndToEndStep `json:"steps"`
	Hint      string             `json:"hint,omitempty"`
}

// EndToEndFromProbes derives end-to-end workflow status from an existing probe bundle (no extra RPC).
func EndToEndFromProbes(probes CoreProbesBundle) CoreEndToEndProbeResult {
	checkedAt := probes.CheckedAt
	if checkedAt == "" {
		checkedAt = time.Now().UTC().Format(time.RFC3339)
	}
	steps := []CoreEndToEndStep{
		{Name: "node_health", OK: maintenanceNodeReady(probes.Maintenance), Note: nodeHealthNote(probes.Maintenance)},
		{Name: "restart_resume", OK: probes.RestartResume.OK, Note: restartResumeE2ENote(probes.RestartResume)},
		{Name: "ibd_convergence", OK: ibdConvergenceE2EOK(probes.IbdConvergence), Skipped: probes.IbdConvergence.Skipped, Note: ibdConvergenceE2ENote(probes.IbdConvergence)},
		{Name: "addrman", OK: addrmanE2EOK(probes.Addrman), Skipped: probes.Addrman.Skipped, Note: addrmanE2ENote(probes.Addrman)},
		{Name: "maintenance", OK: probes.Maintenance.OK},
		{Name: "reindex_check", OK: probes.Reindex.OK},
	}
	mp := probes.MempoolParity
	if mp.OfflineCorpus != nil {
		steps = append(steps, CoreEndToEndStep{
			Name: "offline_corpus",
			OK:   mp.OfflineCorpus.OK,
			Note: fmt.Sprintf("%d/%d templates", mp.OfflineCorpus.Passed, mp.OfflineCorpus.Total),
		})
		steps = append(steps, CoreEndToEndStep{
			Name: "bip125_offline",
			OK:   mp.OfflineCorpus.OK,
			Note: "BIP125 rule 2/5 (rbf_too_many_conflicts, rbf_new_unconfirmed_input)",
		})
	}
	if mp.Skipped {
		steps = append(steps, CoreEndToEndStep{
			Name: "mempool_parity", OK: false, Skipped: true, Note: mp.Reason,
		})
	} else if mp.Total == 0 && mp.OfflineCorpus == nil {
		steps = append(steps, CoreEndToEndStep{
			Name: "mempool_parity", OK: false, Skipped: true, Note: "mempool probe not bundled",
		})
	} else {
		mpOK := mp.OK
		if mp.CoreConfigured && mp.CoreAvailable && !mp.CoreAligned {
			mpOK = false
		}
		steps = append(steps, CoreEndToEndStep{
			Name: "mempool_parity", OK: mpOK, Note: mempoolE2ENote(mp),
		})
	}
	steps = append(steps, CoreEndToEndStep{
		Name: "bip152_hb", OK: probes.Bip152.OK, Note: bip152E2ENote(probes.Bip152),
	})
	steps = append(steps, CoreEndToEndStep{
		Name: "pq_format", OK: probes.PQ.OK, Note: pqE2ENote(probes.PQ),
	})
	if probes.Compare.DeploymentChecked {
		note := "deployment.protocol_lock"
		if !probes.Compare.CoreConfigured {
			note = "solo deployment sanity"
		}
		steps = append(steps, CoreEndToEndStep{
			Name: "protocol_lock", OK: probes.Compare.ProtocolLockOK, Note: note,
		})
	}
	sp := probes.SetupParity
	if sp.Skipped {
		steps = append(steps, CoreEndToEndStep{
			Name: "setup_parity", OK: true, Skipped: true, Note: sp.SkipReason,
		})
	} else {
		steps = append(steps, CoreEndToEndStep{
			Name: "setup_parity", OK: sp.OK, Note: setupParityE2ENote(sp),
		})
	}
	if probes.Wallet.Skipped {
		steps = append(steps, CoreEndToEndStep{Name: "wallet_basics", OK: true, Skipped: true, Note: "wallet not enabled"})
	} else {
		steps = append(steps, CoreEndToEndStep{
			Name: "wallet_basics", OK: probes.Wallet.OK, Note: walletE2ENote(probes.Wallet),
		})
	}
	steps = append(steps, CoreEndToEndStep{
		Name: "mining", OK: miningE2EOK(probes.Mining), Skipped: miningE2ESkipped(probes.Mining), Note: miningE2ENote(probes.Mining),
	})
	return CoreEndToEndProbeResult{
		CheckedAt: checkedAt,
		OK:        endToEndStepsOK(steps),
		Steps:     steps,
		Hint:      "Built-in operator workflow (node health via maintenance RPC, restart-resume, IBD convergence snapshot, addrman, maintenance, reindex, Milestone D offline corpus + BIP125 rule 2/5 + live mempool parity, BIP152 HB, PQ format/carrier, protocol-lock solo sanity, Milestone D setup-parity, wallet when enabled, mining GBT/aux). Optional scripts/core_end_to_end_workflow.ps1 mirrors these steps for Windows CI with dogecoin-cli.",
	}
}

func miningE2ESkipped(m CoreMiningProbeResult) bool {
	return m.CheckedAt == "" && !m.OK && len(m.Issues) == 0 && len(m.Checks) == 0
}

func miningE2EOK(m CoreMiningProbeResult) bool {
	if miningE2ESkipped(m) {
		return true
	}
	return m.OK
}

func miningE2ENote(m CoreMiningProbeResult) string {
	if miningE2ESkipped(m) {
		return "mining probe not bundled"
	}
	var parts []string
	if m.GBTFieldsOK {
		parts = append(parts, "gbt ok")
	}
	if m.CreateAuxOK {
		parts = append(parts, "createaux ok")
	} else if m.CreateAuxSkipped {
		parts = append(parts, "pre-aux")
	}
	if m.CoreAligned {
		parts = append(parts, "core aligned")
	}
	if !m.OK && len(m.Issues) > 0 {
		return m.Issues[0]
	}
	return strings.Join(parts, " · ")
}

func nodeHealthNote(m CoreMaintenanceResult) string {
	for _, c := range m.Checks {
		if c.Name == "getblockchaininfo" && c.Status == "issue" {
			return c.Note
		}
	}
	if m.IBD && len(m.Issues) > 0 {
		return "syncing (IBD)"
	}
	return ""
}

func maintenanceNodeReady(m CoreMaintenanceResult) bool {
	return maintenanceOperationalOK(m)
}

func setupParityE2ENote(sp SetupParityProbeResult) string {
	var parts []string
	if sp.CLI != "" {
		parts = append(parts, sp.CLI)
	}
	if sp.Setup.DogeGoBalance > 0 {
		parts = append(parts, fmt.Sprintf("dogego_balance=%g", sp.Setup.DogeGoBalance))
	}
	if sp.Setup.CoreBalance != nil {
		parts = append(parts, fmt.Sprintf("core_balance=%g", *sp.Setup.CoreBalance))
	}
	return strings.Join(parts, " ")
}

func mempoolE2ENote(mp MempoolParityProbeResult) string {
	var parts []string
	if mp.Total > 0 {
		parts = append(parts, fmt.Sprintf("stateless %d/%d", mp.Passed, mp.Total))
	}
	if mp.CoreConfigured && mp.CoreAvailable && !mp.CoreAligned {
		parts = append(parts, "core drift")
	}
	if mp.OfflineStateful != nil && mp.OfflineStateful.Total > 0 {
		parts = append(parts, fmt.Sprintf("stateful %d/%d", mp.OfflineStateful.Passed, mp.OfflineStateful.Total))
	}
	return strings.Join(parts, " ")
}

func walletE2ENote(w CoreWalletProbeResult) string {
	var parts []string
	if w.WalletHistoryDeferReason != "" {
		parts = append(parts, "defer="+w.WalletHistoryDeferReason)
	}
	if w.WalletListTransactionsMs > 0 {
		parts = append(parts, fmt.Sprintf("listtransactions_40=%dms", w.WalletListTransactionsMs))
	}
	if w.WalletListTransactionsMs > 0 && !w.WalletListTransactionsOK {
		parts = append(parts, "slow")
	}
	if w.PoolReplayScanCap > 0 {
		parts = append(parts, fmt.Sprintf("pool_replay_scan=%d", w.PoolReplayScanCap))
	}
	if w.WalletTxHexOK {
		parts = append(parts, "tx_hex_ok")
	}
	if w.WalletPqSendOK {
		parts = append(parts, "pq_send_ok")
		if w.WalletPqTag != "" {
			parts = append(parts, w.WalletPqTag)
		}
	}
	return strings.Join(parts, " ")
}

func ibdConvergenceE2EOK(ic CoreIbdConvergenceProbeResult) bool {
	if ic.Skipped {
		return true
	}
	return ic.OK
}

func ibdConvergenceE2ENote(ic CoreIbdConvergenceProbeResult) string {
	if ic.Skipped {
		if ic.Reason != "" {
			return ic.Reason
		}
		return "skipped"
	}
	var parts []string
	if ic.BodyCoveragePct > 0 {
		parts = append(parts, fmt.Sprintf("coverage=%.1f%%", ic.BodyCoveragePct))
	}
	if ic.ConnectBoost != "" {
		parts = append(parts, "boost="+ic.ConnectBoost)
	}
	if len(ic.Notes) > 0 {
		parts = append(parts, ic.Notes[0])
	}
	if len(ic.Issues) > 0 {
		parts = append(parts, ic.Issues[0])
	}
	return strings.Join(parts, " ")
}

func addrmanE2EOK(am CoreAddrmanProbeResult) bool {
	if am.Skipped {
		return true
	}
	return am.OK
}

func addrmanE2ENote(am CoreAddrmanProbeResult) string {
	if am.Skipped {
		if am.Reason != "" {
			return am.Reason
		}
		return "skipped"
	}
	if am.Tried != nil && am.NewAddrs != nil {
		return fmt.Sprintf("tried=%d new=%d", *am.Tried, *am.NewAddrs)
	}
	if len(am.Notes) > 0 {
		return am.Notes[0]
	}
	if len(am.Issues) > 0 {
		return am.Issues[0]
	}
	return ""
}

func bip152E2ENote(bp CoreBip152ProbeResult) string {
	if bp.Skipped {
		return bp.Reason
	}
	if len(bp.Notes) > 0 {
		return bp.Notes[0]
	}
	if len(bp.Issues) > 0 {
		return bp.Issues[0]
	}
	return ""
}

func pqE2ENote(pq CorePQProbeResult) string {
	if len(pq.Checks) == 0 {
		if pq.OK {
			return "pq checks ok"
		}
		return "pq checks failed"
	}
	ok := 0
	for _, c := range pq.Checks {
		if c.Status == "ok" {
			ok++
		}
	}
	return fmt.Sprintf("%d/%d checks ok", ok, len(pq.Checks))
}

func restartResumeE2ENote(rr CoreRestartResumeResult) string {
	var parts []string
	if rr.ConnectLag > 0 {
		parts = append(parts, fmt.Sprintf("connect_lag=%d", rr.ConnectLag))
	}
	if rr.ConnectCatchUpPasses > 0 && rr.ConnectCatchUpBatch > 0 {
		boost := fmt.Sprintf("boost=%dx%d", rr.ConnectCatchUpPasses, rr.ConnectCatchUpBatch)
		if rr.ConnectCatchUpIntervalMs > 0 {
			boost += fmt.Sprintf("@%dms", rr.ConnectCatchUpIntervalMs)
		}
		parts = append(parts, boost)
	}
	return strings.Join(parts, " ")
}

func endToEndStepsOK(steps []CoreEndToEndStep) bool {
	for _, s := range steps {
		if s.Skipped {
			continue
		}
		if !s.OK {
			return false
		}
	}
	return true
}

// ProbeCoreEndToEnd runs the bundled workflow probe (fresh RPC).
func ProbeCoreEndToEnd(network, dogeRPCAddr, chainDataDir string, conf config.File, invoke func(string, []json.RawMessage) map[string]interface{}) CoreEndToEndProbeResult {
	probes := RunCoreProbes(network, dogeRPCAddr, chainDataDir, conf, invoke)
	return EndToEndFromProbes(probes)
}
