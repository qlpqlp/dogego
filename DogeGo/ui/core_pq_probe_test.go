// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"

	"dogego/consensus"
)

func TestProbeCorePQOK(t *testing.T) {
	out := ProbeCorePQ()
	if !out.OK {
		t.Fatalf("ok=false issues=%v checks=%+v", out.Issues, out.Checks)
	}
	wantOK := map[string]bool{
		"op_return_" + consensus.PQTagFalcon:    false,
		"op_return_" + consensus.PQTagDilithium: false,
		"op_return_" + consensus.PQTagRaccoon:   false,
		"carrier_" + consensus.PQTagFalcon:      false,
		"carrier_" + consensus.PQTagDilithium:   false,
		"mempool_corpus_pq":                     false,
	}
	raccoonCarrier := ""
	for _, c := range out.Checks {
		if c.Name == "carrier_"+consensus.PQTagRaccoon {
			raccoonCarrier = c.Status
			continue
		}
		if _, ok := wantOK[c.Name]; ok {
			if c.Status != "ok" {
				t.Fatalf("check %s status=%s note=%s", c.Name, c.Status, c.Note)
			}
			wantOK[c.Name] = true
		}
	}
	for name, seen := range wantOK {
		if !seen {
			t.Fatalf("missing check %s", name)
		}
	}
	if raccoonCarrier != "ok" && raccoonCarrier != "warning" {
		t.Fatalf("carrier_RCG4 status=%q want ok or warning", raccoonCarrier)
	}
}
