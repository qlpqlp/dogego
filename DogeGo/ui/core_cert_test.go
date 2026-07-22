// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"strings"
	"testing"

	"dogego/config"
)

func TestDefaultCoreCertManifest(t *testing.T) {
	m := DefaultCoreCertManifest()
	if len(m.Milestones) < 4 {
		t.Fatalf("milestones=%d", len(m.Milestones))
	}
	if m.Corpus.MempoolVectors < 58 {
		t.Fatalf("corpus mempool=%d want >=58", m.Corpus.MempoolVectors)
	}
	var milestoneE *CertificationMilestone
	for i := range m.Milestones {
		if m.Milestones[i].ID == "milestone_e" {
			milestoneE = &m.Milestones[i]
			break
		}
	}
	if milestoneE == nil || len(milestoneE.OfflineTests) < 3 {
		t.Fatalf("milestone_e offline tests=%v", milestoneE)
	}
	foundProbe := false
	foundAddressAPI := false
	for _, name := range milestoneE.OfflineTests {
		if name == "TestProbeCoreWalletOk" {
			foundProbe = true
		}
		if name == "TestWalletAddressLabelAPI" {
			foundAddressAPI = true
		}
	}
	if !foundProbe {
		t.Fatal("milestone_e missing TestProbeCoreWalletOk")
	}
	if !foundAddressAPI {
		t.Fatal("milestone_e missing TestWalletAddressLabelAPI")
	}
}

func TestDefaultCoreCertManifestHarnessCommands(t *testing.T) {
	m := DefaultCoreCertManifest()
	if !strings.Contains(m.Disclaimer, "no protocol fork") {
		t.Fatalf("disclaimer missing protocol lock: %q", m.Disclaimer)
	}
	want := []string{
		"dogego cert offline",
		"cert_offline_prerequisites",
		"dogego cert operator",
		"dogego cert field-evidence",
		"dogego cert wallet-migration",
		"dogego cert wallet-import",
		"dogego cert pq",
		"dogego cert provision -preflight",
		"dogego cert weekly -require-wallet-dat",
		"dogego cert weekly-live",
		"dogego cert enable-weekly",
	}
	for _, cmd := range want {
		found := false
		for _, h := range m.HarnessCommands {
			if h == cmd || strings.HasPrefix(h, cmd+" ") || strings.Contains(h, cmd) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("HarnessCommands missing %q", cmd)
		}
	}
}

func TestMilestoneCanonicalCertScriptPaths(t *testing.T) {
	m := DefaultCoreCertManifest()
	paths := map[string]string{
		"milestone_a": "fieldevidence/suites.go",
		"milestone_e": "offlinegate/suites.go",
	}
	for id, wantPath := range paths {
		var ms *CertificationMilestone
		for i := range m.Milestones {
			if m.Milestones[i].ID == id {
				ms = &m.Milestones[i]
				break
			}
		}
		if ms == nil {
			t.Fatalf("missing milestone %q", id)
		}
		found := false
		for _, s := range ms.Scripts {
			if s.Path == wantPath {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("milestone %q missing script path %q", id, wantPath)
		}
	}
}

func TestMilestoneCoreCertMilestoneBScripts(t *testing.T) {
	m := DefaultCoreCertManifest()
	paths := map[string]string{
		"milestone_b": "runner/live_soak.go",
		"milestone_d": "runner/weekly_live.go",
	}
	for id, wantPath := range paths {
		var ms *CertificationMilestone
		for i := range m.Milestones {
			if m.Milestones[i].ID == id {
				ms = &m.Milestones[i]
				break
			}
		}
		if ms == nil {
			t.Fatalf("missing milestone %q", id)
		}
		found := false
		for _, s := range ms.Scripts {
			if s.Path == wantPath {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("milestone %q missing script path %q", id, wantPath)
		}
	}
}

func TestMilestoneCoreCertWalletImportScript(t *testing.T) {
	m := DefaultCoreCertManifest()
	var ms *CertificationMilestone
	for i := range m.Milestones {
		if m.Milestones[i].ID == "milestone_e" {
			ms = &m.Milestones[i]
			break
		}
	}
	if ms == nil {
		t.Fatal("missing milestone_e")
	}
	for _, wantPath := range []string{"walletimport/verify.go", "walletmigration/verify.go", "operatorworkflow/verify.go", "pqcert/suites.go"} {
		found := false
		for _, s := range ms.Scripts {
			if s.Path == wantPath {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("milestone_e missing script path %q", wantPath)
		}
	}
}

func TestDefaultCoreRPCAddr(t *testing.T) {
	if DefaultCoreRPCAddr("mainnet") != "127.0.0.1:22555" {
		t.Fatal("mainnet core addr")
	}
	if DefaultCoreRPCAddr("testnet") != "127.0.0.1:44556" {
		t.Fatal("testnet core addr")
	}
}

func TestProbeCoreCompareNoInvoke(t *testing.T) {
	r := ProbeCoreCompare("mainnet", "127.0.0.1:22557", config.File{}, nil)
	if len(r.Errors) == 0 {
		t.Fatal("expected dogego error")
	}
	if r.VerifyOK {
		t.Fatal("verify_ok should be false when probe aborts early")
	}
}

func TestCoreCompareHelpers(t *testing.T) {
	if !boolFromAny(true) {
		t.Fatal("boolFromAny true")
	}
	if boolFromAny(false) {
		t.Fatal("boolFromAny false")
	}
	n, ok := intFromAny(float64(42))
	if !ok || n != 42 {
		t.Fatalf("intFromAny float: %d %v", n, ok)
	}
}
