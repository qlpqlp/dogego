// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "testing"

func TestProbeCoreFieldEvidenceOfflineCorpus(t *testing.T) {
	out := ProbeCoreFieldEvidence("mainnet")
	if !out.OK {
		t.Fatal("offline corpus should always be ok")
	}
	if len(out.Checks) < 2 {
		t.Fatalf("checks=%+v", out.Checks)
	}
	if out.Checks[0].Name != "offline_corpus" || out.Checks[0].Status != "ok" {
		t.Fatalf("first check=%+v", out.Checks[0])
	}
}

func TestProbeCoreFieldEvidenceTestnetMainnetOnlyNote(t *testing.T) {
	out := ProbeCoreFieldEvidence("testnet")
	found := false
	for _, n := range out.Notes {
		if n == "milestone_a_mainnet_only" {
			found = true
		}
	}
	if !found {
		t.Fatalf("notes=%v", out.Notes)
	}
}
