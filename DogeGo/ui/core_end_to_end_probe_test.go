// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"dogego/config"
	"dogego/runner"
)

func TestEndToEndFromProbes(t *testing.T) {
	probes := CoreProbesBundle{
		CheckedAt: "2026-01-01T00:00:00Z",
		Maintenance: CoreMaintenanceResult{
			OK: true,
			Checks: []CoreMaintenanceCheck{{Name: "getblockchaininfo", Status: "ok"}},
		},
		RestartResume:  CoreRestartResumeResult{OK: true},
		IbdConvergence: CoreIbdConvergenceProbeResult{OK: true},
		Addrman:        CoreAddrmanProbeResult{Skipped: true},
		Reindex:        CoreReindexProbeResult{OK: true},
		Bip152:        CoreBip152ProbeResult{OK: true, SchemaOK: true},
		PQ:            CorePQProbeResult{OK: true, Checks: []CorePQProbeCheck{{Name: "flc1", Status: "ok"}}},
		MempoolParity: MempoolParityProbeResult{
			OK: true, Total: 32, Passed: 32,
			OfflineCorpus:   &MempoolOfflineCorpusSummary{OK: true, Total: 58, Passed: 58},
			OfflineStateful: &MempoolOfflineStatefulSummary{OK: true, Total: 26, Passed: 26},
		},
		SetupParity: SetupParityProbeResult{Skipped: true, OK: true, SkipReason: "not reboot testnet"},
		Wallet:      CoreWalletProbeResult{Skipped: true},
	}
	out := EndToEndFromProbes(probes)
	if !out.OK {
		t.Fatalf("ok=false steps=%+v", out.Steps)
	}
	if len(out.Steps) < 10 {
		t.Fatalf("steps=%d", len(out.Steps))
	}
	var sawPQ bool
	for _, s := range out.Steps {
		if s.Name == "pq_format" {
			sawPQ = true
			if !s.OK || !strings.Contains(s.Note, "checks ok") {
				t.Fatalf("pq_format step=%+v", s)
			}
		}
	}
	if !sawPQ {
		t.Fatal("missing pq_format step")
	}
}

func TestEndToEndFromProbesWalletDeferNote(t *testing.T) {
	out := EndToEndFromProbes(CoreProbesBundle{
		Maintenance:    CoreMaintenanceResult{OK: true},
		RestartResume:  CoreRestartResumeResult{OK: true},
		IbdConvergence: CoreIbdConvergenceProbeResult{OK: true},
		Addrman:        CoreAddrmanProbeResult{Skipped: true},
		Reindex:        CoreReindexProbeResult{OK: true},
		Bip152:         CoreBip152ProbeResult{OK: true},
		SetupParity:    SetupParityProbeResult{Skipped: true, OK: true},
		Wallet: CoreWalletProbeResult{
			OK:                       true,
			WalletHistoryDeferred:    true,
			WalletHistoryDeferReason: "scan_building",
		},
	})
	var note string
	for _, s := range out.Steps {
		if s.Name == "wallet_basics" {
			note = s.Note
			break
		}
	}
	if !strings.Contains(note, "defer=scan_building") {
		t.Fatalf("note=%q", note)
	}
}

func TestEndToEndFromProbesMempoolSteps(t *testing.T) {
	out := EndToEndFromProbes(CoreProbesBundle{
		Maintenance:    CoreMaintenanceResult{OK: true},
		RestartResume:  CoreRestartResumeResult{OK: true},
		Addrman:        CoreAddrmanProbeResult{Skipped: true},
		Reindex:       CoreReindexProbeResult{OK: true},
		Bip152:        CoreBip152ProbeResult{OK: true},
		MempoolParity: MempoolParityProbeResult{
			OK: true, Total: 32, Passed: 32,
			OfflineCorpus: &MempoolOfflineCorpusSummary{OK: true, Total: 58, Passed: 58},
		},
		SetupParity: SetupParityProbeResult{Skipped: true, OK: true},
		Wallet:      CoreWalletProbeResult{Skipped: true},
	})
	var corpus, parity, bip125 bool
	for _, s := range out.Steps {
		switch s.Name {
		case "offline_corpus":
			corpus = s.OK && strings.Contains(s.Note, "58/58")
		case "bip125_offline":
			bip125 = s.OK && strings.Contains(s.Note, "rbf_too_many_conflicts")
		case "mempool_parity":
			parity = s.OK && strings.Contains(s.Note, "stateless 32/32")
		}
	}
	if !corpus || !parity || !bip125 {
		t.Fatalf("steps=%+v", out.Steps)
	}
}

func TestEndToEndFromProbesIbdStep(t *testing.T) {
	out := EndToEndFromProbes(CoreProbesBundle{
		Maintenance:    CoreMaintenanceResult{OK: true},
		RestartResume:  CoreRestartResumeResult{OK: true},
		IbdConvergence: CoreIbdConvergenceProbeResult{OK: true, BodyCoveragePct: 42.5, Notes: []string{"forward_ibd_active"}},
		Addrman:        CoreAddrmanProbeResult{Skipped: true},
		Reindex:        CoreReindexProbeResult{OK: true},
		Bip152:         CoreBip152ProbeResult{OK: true},
		MempoolParity:  MempoolParityProbeResult{Skipped: true, Reason: "warming up"},
		SetupParity:    SetupParityProbeResult{Skipped: true, OK: true},
		Wallet:         CoreWalletProbeResult{Skipped: true},
	})
	for _, s := range out.Steps {
		if s.Name == "ibd_convergence" && s.OK && strings.Contains(s.Note, "coverage=42.5%") {
			return
		}
	}
	t.Fatalf("steps=%+v", out.Steps)
}

func TestEndToEndFromProbesFailStep(t *testing.T) {
	probes := CoreProbesBundle{
		Maintenance:   CoreMaintenanceResult{OK: true},
		RestartResume: CoreRestartResumeResult{OK: false},
		Addrman:       CoreAddrmanProbeResult{Skipped: true},
		Reindex:       CoreReindexProbeResult{OK: true},
		Bip152:        CoreBip152ProbeResult{OK: true},
		SetupParity:   SetupParityProbeResult{Skipped: true, OK: true},
		Wallet:        CoreWalletProbeResult{Skipped: true},
	}
	out := EndToEndFromProbes(probes)
	if out.OK {
		t.Fatal("expected fail")
	}
}

func TestEndToEndFromProbesRestartResumeNote(t *testing.T) {
	probes := CoreProbesBundle{
		Maintenance: CoreMaintenanceResult{OK: true},
		RestartResume: CoreRestartResumeResult{
			OK:                       true,
			ConnectLag:               9000,
			ConnectCatchUpPasses:     8,
			ConnectCatchUpBatch:      128,
			ConnectCatchUpIntervalMs: 500,
		},
		Addrman:     CoreAddrmanProbeResult{Skipped: true},
		Reindex:     CoreReindexProbeResult{OK: true},
		SetupParity: SetupParityProbeResult{Skipped: true, OK: true},
		Wallet:      CoreWalletProbeResult{Skipped: true},
	}
	out := EndToEndFromProbes(probes)
	var note string
	for _, s := range out.Steps {
		if s.Name == "restart_resume" {
			note = s.Note
			break
		}
	}
	if note == "" || !strings.Contains(note, "connect_lag=9000") || !strings.Contains(note, "boost=8x128") {
		t.Fatalf("note=%q", note)
	}
}

func TestApplyCoreOperatorCertEndToEndRow(t *testing.T) {
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), CoreProbesBundle{
		Maintenance:   CoreMaintenanceResult{OK: true},
		RestartResume: CoreRestartResumeResult{OK: true},
		Addrman:       CoreAddrmanProbeResult{Skipped: true},
		Reindex:       CoreReindexProbeResult{OK: true},
		Bip152:        CoreBip152ProbeResult{OK: true, SchemaOK: true},
		SetupParity:   SetupParityProbeResult{Skipped: true, OK: true},
		Wallet:        CoreWalletProbeResult{Skipped: true},
		EndToEnd:      CoreEndToEndProbeResult{OK: true, CheckedAt: "2026-01-01T00:00:00Z"},
	})
	var e2e *CoreOperatorCertRow
	for i := range rows {
		if rows[i].ID == "end_to_end" {
			e2e = &rows[i]
			break
		}
	}
	if e2e == nil || e2e.OK == nil || !*e2e.OK {
		t.Fatalf("end_to_end row: %+v", e2e)
	}
}

func TestProbeCoreEndToEndInvoke(t *testing.T) {
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		switch method {
		case "getblockchaininfo":
			return map[string]interface{}{"blocks": int64(100), "headers": int64(100)}
		case "verifychain":
			return map[string]interface{}{"result": true}
		case "getindexinfo":
			return map[string]interface{}{}
		case "getrpcinfo":
			return map[string]interface{}{}
		case "getwalletinfo":
			return nil
		case "getpeerinfo":
			return map[string]interface{}{
				"result": []interface{}{
					map[string]interface{}{
						"addr":           "peer:1",
						"bip152_hb_to":   true,
						"bip152_hb_from": false,
					},
				},
			}
		default:
			return map[string]interface{}{}
		}
	}
	out := ProbeCoreEndToEnd("mainnet", "", t.TempDir(), config.File{}, invoke)
	if len(out.Steps) == 0 {
		t.Fatalf("steps empty")
	}
}

func TestEndToEndFromProbesWalletNote(t *testing.T) {
	out := EndToEndFromProbes(CoreProbesBundle{
		Maintenance:   CoreMaintenanceResult{OK: true},
		RestartResume: CoreRestartResumeResult{OK: true},
		Addrman:       CoreAddrmanProbeResult{Skipped: true},
		Reindex:       CoreReindexProbeResult{OK: true},
		Bip152:        CoreBip152ProbeResult{OK: true},
		SetupParity:   SetupParityProbeResult{Skipped: true, OK: true},
		Wallet: CoreWalletProbeResult{
			OK: true, WalletListTransactionsMs: 42, WalletListTransactionsOK: true, WalletTxHexOK: true,
		},
	})
	var note string
	for _, s := range out.Steps {
		if s.Name == "wallet_basics" {
			note = s.Note
			break
		}
	}
	if !strings.Contains(note, "listtransactions_40=42ms") || !strings.Contains(note, "tx_hex_ok") {
		t.Fatalf("note=%q", note)
	}
}

func TestEndToEndFromProbesProtocolLockStep(t *testing.T) {
	out := EndToEndFromProbes(CoreProbesBundle{
		Maintenance:   CoreMaintenanceResult{OK: true},
		RestartResume: CoreRestartResumeResult{OK: true},
		Addrman:       CoreAddrmanProbeResult{Skipped: true},
		Reindex:       CoreReindexProbeResult{OK: true},
		Bip152:        CoreBip152ProbeResult{OK: true, SchemaOK: true},
		SetupParity:   SetupParityProbeResult{Skipped: true, OK: true},
		Wallet:        CoreWalletProbeResult{Skipped: true},
		Compare: CoreCompareResult{
			DeploymentChecked: true,
			ProtocolLockOK:    true,
		},
	})
	var found bool
	for _, s := range out.Steps {
		if s.Name == "protocol_lock" {
			found = true
			if !s.OK {
				t.Fatal("expected protocol_lock OK")
			}
		}
	}
	if !found {
		t.Fatal("missing protocol_lock step")
	}
}

func TestEndToEndFromProbesSetupParityStep(t *testing.T) {
	out := EndToEndFromProbes(CoreProbesBundle{
		Maintenance:   CoreMaintenanceResult{OK: true},
		RestartResume: CoreRestartResumeResult{OK: true},
		Addrman:       CoreAddrmanProbeResult{Skipped: true},
		Reindex:       CoreReindexProbeResult{OK: true},
		Bip152:        CoreBip152ProbeResult{OK: true},
		SetupParity: SetupParityProbeResult{
			OK: true, CLI: "dogego cert setup-parity -mine-bootstrap",
			Setup: runner.SetupParityResult{DogeGoBalance: 12.5},
		},
		Wallet: CoreWalletProbeResult{Skipped: true},
	})
	var found bool
	for _, s := range out.Steps {
		if s.Name == "setup_parity" {
			found = true
			if !s.OK || !strings.Contains(s.Note, "dogego_balance=12.5") {
				t.Fatalf("step=%+v", s)
			}
		}
	}
	if !found {
		t.Fatal("missing setup_parity step")
	}
}
