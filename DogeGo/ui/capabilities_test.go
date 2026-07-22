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
)

func TestDefaultCapabilitiesManifest(t *testing.T) {
	m := DefaultCapabilitiesManifest()
	if m.ClientVersion == "" {
		t.Fatal("client version")
	}
	if len(m.Categories) < 6 {
		t.Fatalf("categories=%d", len(m.Categories))
	}
	if len(m.CoreParityGaps) < 20 {
		t.Fatalf("core parity gaps=%d", len(m.CoreParityGaps))
	}
	if len(m.RPCMethods) < 50 {
		t.Fatalf("rpc methods=%d", len(m.RPCMethods))
	}
	if m.ParitySummary.RoadmapTotal < 10 {
		t.Fatalf("roadmap total=%d", m.ParitySummary.RoadmapTotal)
	}
	if m.ParitySummary.FeaturesLive < 5 {
		t.Fatalf("features live=%d", m.ParitySummary.FeaturesLive)
	}
	if m.ParitySummary.ExitGateStatus != "ready" {
		t.Fatalf("exit gate=%q", m.ParitySummary.ExitGateStatus)
	}
	if len(m.CoreGuidance.UseDogeGoWhen) < 2 {
		t.Fatal("core guidance lists")
	}
	if len(m.Certification.Milestones) < 4 {
		t.Fatalf("cert milestones=%d", len(m.Certification.Milestones))
	}
	if len(m.CoreProbeAPIs) < 5 {
		t.Fatalf("core probe apis=%d", len(m.CoreProbeAPIs))
	}
	for _, row := range m.RPCMethods {
		if row.Method == "" || row.Class == "" {
			t.Fatalf("bad row %#v", row)
		}
	}
}

func TestClassifyRPCHelp(t *testing.T) {
	if classifyRPCHelp("not implemented") != "stub" {
		t.Fatal("stub")
	}
	if classifyRPCHelp("DogeGo: no wallet") != "partial" {
		t.Fatal("partial")
	}
	if classifyRPCHelp("Returns the hash of the best block") != "live" {
		t.Fatal("live")
	}
}

func TestDefaultCoreParityGaps(t *testing.T) {
	gaps := DefaultCoreParityGaps()
	if len(gaps) < 20 {
		t.Fatalf("gaps=%d", len(gaps))
	}
	seen := make(map[string]bool)
	for _, g := range gaps {
		if g.ID == "" || g.Title == "" || g.Area == "" {
			t.Fatalf("bad gap %#v", g)
		}
		if seen[g.ID] {
			t.Fatalf("duplicate gap id %q", g.ID)
		}
		seen[g.ID] = true
	}
	if !seen["protocol_lock"] {
		t.Fatal("missing protocol_lock gap")
	}
	if g := gapsByID(gaps, "protocol_lock"); g == nil || g.Status != "done" {
		t.Fatalf("protocol_lock status=%v", g)
	} else if !strings.Contains(g.Summary, "no consensus fork") && !strings.Contains(g.Summary, "protocol fork") {
		t.Fatalf("protocol_lock summary=%q", g.Summary)
	}
	if !seen["dogego_live_scheduled_ci"] {
		t.Fatal("missing dogego_live_scheduled_ci gap")
	}
	if g := gapsByID(gaps, "dogego_live_scheduled_ci"); g == nil || !strings.Contains(g.Summary, "enable-weekly") {
		t.Fatalf("dogego_live_scheduled_ci summary=%v", g)
	}
	if g := gapsByID(gaps, "wallet_keypool"); g == nil || g.Status != "done" {
		t.Fatalf("wallet_keypool status=%v", g)
	} else if !strings.Contains(g.Summary, "pool_indices_replayed") {
		t.Fatalf("wallet_keypool summary=%q", g.Summary)
	}
	if g := gapsByID(gaps, "wallet_core_migration"); g == nil || g.Status != "done" {
		t.Fatalf("wallet_core_migration status=%v", g)
	} else if !strings.Contains(g.Summary, "pool_indices_replayed") || !strings.Contains(g.Summary, "pool_replay") || !strings.Contains(g.Summary, "wallet-import") {
		t.Fatalf("wallet_core_migration summary=%q", g.Summary)
	}
	if g := gapsByID(gaps, "milestone_e_full"); g == nil {
		t.Fatal("missing milestone_e_full gap")
	} else if !strings.Contains(strings.ToLower(g.Summary), "seventeen live") || !strings.Contains(g.Summary, "enable-weekly") {
		t.Fatalf("milestone_e_full summary=%q", g.Summary)
	}
	if g := gapsByID(gaps, "mining_gbt_aux"); g == nil || g.Status != "done" {
		t.Fatalf("mining_gbt_aux status=%v", g)
	} else if !strings.Contains(g.Summary, "core-mining-probe") {
		t.Fatalf("mining_gbt_aux summary=%q", g.Summary)
	}
	if g := gapsByID(gaps, "pq_verify"); g == nil || g.Status != "done" {
		t.Fatalf("pq_verify status=%v", g)
	} else if !strings.Contains(g.Summary, "dogego cert pq") {
		t.Fatalf("pq_verify summary=%q", g.Summary)
	}
}

func gapsByID(gaps []CoreParityGap, id string) *CoreParityGap {
	for i := range gaps {
		if gaps[i].ID == id {
			return &gaps[i]
		}
	}
	return nil
}

func TestDefaultCoreProbeAPIsMilestoneDPaths(t *testing.T) {
	apis := defaultCoreProbeAPIs()
	want := map[string]bool{
		"/api/mempool/stateful-status": false,
		"/api/core-setup-parity":       false,
	}
	for i := range apis {
		if _, ok := want[apis[i].Path]; ok {
			want[apis[i].Path] = true
		}
	}
	for path, found := range want {
		if !found {
			t.Fatalf("missing probe api %s", path)
		}
	}
}

func TestDefaultCoreProbeAPIsRunnerBundled(t *testing.T) {
	apis := defaultCoreProbeAPIs()
	var runner *CoreProbeAPI
	for i := range apis {
		if apis[i].Path == "/api/core-runner-probes" {
			runner = &apis[i]
			break
		}
	}
	if runner == nil || !runner.Bundled {
		t.Fatalf("runner probe api=%+v", runner)
	}
}

func TestDefaultCoreProbeAPIsWorkflow10Bundled(t *testing.T) {
	apis := defaultCoreProbeAPIs()
	var wf *CoreProbeAPI
	for i := range apis {
		if apis[i].Path == "/api/core-workflow10-probe" {
			wf = &apis[i]
			break
		}
	}
	if wf == nil || !wf.Bundled {
		t.Fatalf("workflow10 probe api=%+v", wf)
	}
}

func TestCapabilitiesJSON(t *testing.T) {
	m := DefaultCapabilitiesManifest()
	EnrichCapabilitiesLive(&m, map[string]any{"rpc_enabled": true})
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 500 {
		t.Fatalf("short json %d", len(b))
	}
}
