// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"time"

	"dogego/config"
)

// CoreProbesBundle aggregates all live Core parity probes (Features tab bundle).
type CoreProbesBundle struct {
	CheckedAt     string                    `json:"checked_at"`
	Compare       CoreCompareResult         `json:"compare"`
	Maintenance   CoreMaintenanceResult     `json:"maintenance"`
	RestartResume CoreRestartResumeResult   `json:"restart_resume"`
	MempoolParity MempoolParityProbeResult  `json:"mempool_parity"`
	Wallet        CoreWalletProbeResult     `json:"wallet"`
	Reindex       CoreReindexProbeResult    `json:"reindex"`
	Autostart     AutostartLoginProbeResult `json:"autostart"`
	Founder       FounderProbeResult        `json:"founder"`
	EndToEnd      CoreEndToEndProbeResult   `json:"end_to_end"`
	Runner        RunnerProbesResult        `json:"runner"`
	SetupParity   SetupParityProbeResult    `json:"setup_parity"`
	Bip152         CoreBip152ProbeResult          `json:"bip152"`
	Addrman        CoreAddrmanProbeResult         `json:"addrman"`
	Mining         CoreMiningProbeResult          `json:"mining"`
	Workflow10     Workflow10ProbeResult          `json:"workflow10"`
	IbdConvergence CoreIbdConvergenceProbeResult  `json:"ibd_convergence"`
	PQ             CorePQProbeResult              `json:"pq"`
}

// runnerProbeOptionsForConf sets dogego-live preflight strictness: require Core only when explicitly configured.
func runnerProbeOptionsForConf(network string, conf config.File) RunnerProbeOptions {
	return RunnerProbeOptions{RequireCore: CoreCompareEnabled(network, conf)}
}

// RunCoreProbes executes compare, maintenance, restart-resume, mempool parity, wallet, and reindex probes.
func RunCoreProbes(network, dogeRPCAddr, chainDataDir string, conf config.File, invoke func(string, []json.RawMessage) map[string]interface{}) CoreProbesBundle {
	probes := CoreProbesBundle{
		CheckedAt:     time.Now().UTC().Format(time.RFC3339),
		Compare:       ProbeCoreCompare(network, dogeRPCAddr, conf, invoke),
		Maintenance:   ProbeCoreMaintenance(network, conf, invoke),
		RestartResume: ProbeCoreRestartResume(network, chainDataDir, conf, invoke),
		MempoolParity: RunMempoolParityProbe(network, conf, invoke),
		Wallet:        ProbeCoreWallet(invoke),
		Reindex:       ProbeCoreReindex(network, conf, invoke),
		Autostart:     ProbeAutostartLogin(conf),
		Founder:       ProbeFounder(network, conf),
	}
	probes.Runner = ProbeRunnerForNetwork(network, runnerProbeOptionsForConf(network, conf))
	probes.SetupParity = ProbeSetupParity(network)
	probes.Bip152 = ProbeCoreBip152(network, conf, invoke)
	probes.Addrman = ProbeCoreAddrman(invoke)
	probes.Mining = ProbeCoreMining(network, conf, invoke)
	probes.Workflow10 = ProbeWorkflow10ForNetwork(network, Workflow10ProbeOptions{
		SkipProvision: true,
		MineBootstrap: true,
	})
	probes.IbdConvergence = ProbeCoreIbdConvergence(network, chainDataDir, dogeRPCAddr, conf)
	probes.PQ = ProbeCorePQ()
	probes.EndToEnd = EndToEndFromProbes(probes)
	return probes
}
