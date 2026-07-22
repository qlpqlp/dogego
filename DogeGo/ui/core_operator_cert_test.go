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
)

func TestRunCoreOperatorCertRows(t *testing.T) {
	out := RunCoreOperatorCert("mainnet", "", "", config.File{}, nil)
	if len(out.Rows) < 20 {
		t.Fatalf("rows=%d", len(out.Rows))
	}
	web := 0
	for _, r := range out.Rows {
		if r.WebProbe {
			web++
			if r.OK == nil {
				t.Fatalf("web row missing ok: %s", r.ID)
			}
		}
	}
	if web != 17 {
		t.Fatalf("web rows=%d want 17", web)
	}
}

func TestOperatorCertWebGateIDs(t *testing.T) {
	want := []string{
		"core_compare", "maintenance", "restart_resume", "cert_autostart", "cert_founder", "runner_readiness", "restart_connect",
		"setup_reboottestnet_core_parity", "mempool_parity", "reindex", "bip152_relay", "pq_format", "wallet", "mining", "end_to_end", "ibd_converge", "addrman",
	}
	rows := DefaultCoreOperatorCertRows()
	var got []string
	for _, r := range rows {
		if r.WebProbe {
			got = append(got, r.ID)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("gate[%d]=%s want %s", i, got[i], want[i])
		}
	}
}

func TestApplyCoreOperatorCertMempoolCorpusOfflineRow(t *testing.T) {
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), CoreProbesBundle{
		MempoolParity: MempoolParityProbeResult{
			OfflineCorpus: &MempoolOfflineCorpusSummary{OK: true, Total: 58, Passed: 58},
		},
	})
	var row *CoreOperatorCertRow
	for i := range rows {
		if rows[i].ID == "mempool_corpus_offline" {
			row = &rows[i]
			break
		}
	}
	if row == nil || row.OK == nil || !*row.OK {
		t.Fatalf("row: %+v", row)
	}
	if row.Note == "" || !strings.Contains(row.Note, "58/58") {
		t.Fatalf("note=%q", row.Note)
	}
}

func TestApplyCoreOperatorCertRunnerSkippedMainnet(t *testing.T) {
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), CoreProbesBundle{
		Runner: RunnerProbesResult{OK: true, Skipped: true, SkipReason: "not reboot testnet"},
	})
	var rr *CoreOperatorCertRow
	for i := range rows {
		if rows[i].ID == "runner_readiness" {
			rr = &rows[i]
			break
		}
	}
	if rr == nil || rr.OK == nil || !*rr.OK || rr.Note == "" {
		t.Fatalf("runner_readiness row: %+v", rr)
	}
}

func TestApplyCoreOperatorCertRestartConnectNote(t *testing.T) {
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), CoreProbesBundle{
		Maintenance: CoreMaintenanceResult{OK: true},
		RestartResume: CoreRestartResumeResult{
			OK:                       true,
			IBD:                      true,
			ConnectLag:               9000,
			ConnectCatchUpPasses:     8,
			ConnectCatchUpBatch:      128,
			ConnectCatchUpIntervalMs: 500,
		},
		Reindex: CoreReindexProbeResult{OK: true},
		Wallet:  CoreWalletProbeResult{Skipped: true},
	})
	var rc *CoreOperatorCertRow
	for i := range rows {
		if rows[i].ID == "restart_connect" {
			rc = &rows[i]
			break
		}
	}
	if rc == nil || rc.OK == nil || !*rc.OK {
		t.Fatalf("restart_connect row: %+v", rc)
	}
	if rc.Note == "" || !strings.Contains(rc.Note, "boost=8x128") {
		t.Fatalf("note=%q", rc.Note)
	}
}

func TestApplyCoreOperatorCertBip152Relay(t *testing.T) {
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), CoreProbesBundle{
		Bip152: CoreBip152ProbeResult{
			OK: true, SchemaOK: true, HBNegotiated: true,
			HBToPeers: 1, HBFromPeers: 2, PeerCount: 3,
			CmpctRelaySchemaOK: true,
			Notes:              []string{"cmpct_relay_idle"},
		},
	})
	var bp *CoreOperatorCertRow
	for i := range rows {
		if rows[i].ID == "bip152_relay" {
			bp = &rows[i]
			break
		}
	}
	if bp == nil || bp.OK == nil || !*bp.OK {
		t.Fatalf("bip152_relay row: %+v", bp)
	}
	if !strings.Contains(bp.Note, "hb_to=1") || !strings.Contains(bp.Note, "cmpct_schema ok") {
		t.Fatalf("note=%q", bp.Note)
	}
}

func TestApplyCoreOperatorCertPQFormat(t *testing.T) {
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), CoreProbesBundle{
		PQ: ProbeCorePQ(),
	})
	var pq *CoreOperatorCertRow
	for i := range rows {
		if rows[i].ID == "pq_format" {
			pq = &rows[i]
			break
		}
	}
	if pq == nil || pq.OK == nil || !*pq.OK {
		t.Fatalf("pq_format row: %+v", pq)
	}
	if pq.Note == "" || !strings.Contains(pq.Note, "checks ok") {
		t.Fatalf("note=%q", pq.Note)
	}
}

func TestApplyCoreOperatorCertAddrman(t *testing.T) {
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), CoreProbesBundle{
		Addrman: CoreAddrmanProbeResult{
			OK: true, Tried: intPtr(2), NewAddrs: intPtr(5), NKeySet: boolPtr(true),
			BucketSchemaOK: true, Notes: []string{"addrbook tried=2 new=5"},
		},
	})
	var am *CoreOperatorCertRow
	for i := range rows {
		if rows[i].ID == "addrman" {
			am = &rows[i]
			break
		}
	}
	if am == nil || am.OK == nil || !*am.OK {
		t.Fatalf("addrman row: %+v", am)
	}
	if !strings.Contains(am.Note, "tried=2") || !strings.Contains(am.Note, "n_key set") {
		t.Fatalf("note=%q", am.Note)
	}
}

func TestApplyCoreOperatorCertIbdConvergence(t *testing.T) {
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), CoreProbesBundle{
		IbdConvergence: CoreIbdConvergenceProbeResult{
			OK: true, BodyCoveragePct: 88.5, Notes: []string{"forward_ibd_active"},
		},
	})
	var ic *CoreOperatorCertRow
	for i := range rows {
		if rows[i].ID == "ibd_converge" {
			ic = &rows[i]
			break
		}
	}
	if ic == nil || ic.OK == nil || !*ic.OK {
		t.Fatalf("ibd_converge row: %+v", ic)
	}
	if ic.Note == "" {
		t.Fatal("expected ibd_converge note")
	}
}

func intPtr(v int) *int { return &v }

func TestOperatorCertSoloMetricsOptionalCompare(t *testing.T) {
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), CoreProbesBundle{
		Compare:       CoreCompareResult{Available: false, CoreConfigured: false},
		Maintenance:   CoreMaintenanceResult{OK: true},
		RestartResume: CoreRestartResumeResult{OK: true},
		MempoolParity: MempoolParityProbeResult{OK: true, Total: 1, Passed: 1},
		Reindex:       CoreReindexProbeResult{OK: true},
		Bip152:        CoreBip152ProbeResult{OK: true, SchemaOK: true},
		Mining:        CoreMiningProbeResult{OK: true, GBTFieldsOK: true, MiningInfoOK: true, CreateAuxSkipped: true, CheckedAt: "now"},
		PQ:            CorePQProbeResult{OK: true},
		IbdConvergence: CoreIbdConvergenceProbeResult{OK: true},
		Addrman:       CoreAddrmanProbeResult{OK: true},
		SetupParity:   SetupParityProbeResult{Skipped: true, OK: true, SkipReason: "not reboot testnet"},
		Autostart:     AutostartLoginProbeResult{OK: true},
		Founder:       FounderProbeResult{Skipped: true, OK: true, SkipReason: "mainnet"},
		Runner:        RunnerProbesResult{Skipped: true, OK: true, SkipReason: "not reboot testnet"},
		Wallet:        CoreWalletProbeResult{Skipped: true, Reason: "wallet not enabled"},
		EndToEnd:      CoreEndToEndProbeResult{OK: true},
	})
	pass, total, ok := operatorCertSoloMetrics(rows)
	if total != 17 || pass < 4 {
		t.Fatalf("solo pass=%d total=%d ok=%v", pass, total, ok)
	}
	for _, id := range []string{"core_compare", "wallet"} {
		var row *CoreOperatorCertRow
		for i := range rows {
			if rows[i].ID == id {
				row = &rows[i]
				break
			}
		}
		if row == nil || !certRowSoloPass(*row) {
			t.Fatalf("expected solo pass for %s: %+v", id, row)
		}
	}
}

func TestRunCoreOperatorCertWalletSkippedOk(t *testing.T) {
	out := RunCoreOperatorCert("mainnet", "", "", config.File{}, func(method string, params []json.RawMessage) map[string]interface{} {
		if method == "getwalletinfo" {
			return map[string]interface{}{
				"error": map[string]interface{}{"code": float64(-1), "message": "not enabled"},
			}
		}
		return map[string]interface{}{"result": map[string]interface{}{}}
	})
	var wallet *CoreOperatorCertRow
	for i := range out.Rows {
		if out.Rows[i].ID == "wallet" {
			wallet = &out.Rows[i]
			break
		}
	}
	if wallet == nil || wallet.OK == nil || !*wallet.OK {
		t.Fatalf("wallet skipped should be ok: %+v", wallet)
	}
}
