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
	want := map[string]bool{
		"op_return_" + consensus.PQTagFalcon:    false,
		"op_return_" + consensus.PQTagDilithium: false,
		"op_return_" + consensus.PQTagRaccoon:   false,
		"carrier_" + consensus.PQTagFalcon:      false,
		"carrier_" + consensus.PQTagDilithium:   false,
		"carrier_" + consensus.PQTagRaccoon:     false,
		"mempool_corpus_pq":                     false,
	}
	for _, c := range out.Checks {
		if c.Status != "ok" {
			t.Fatalf("check %s status=%s note=%s", c.Name, c.Status, c.Note)
		}
		if _, ok := want[c.Name]; ok {
			want[c.Name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("missing check %s", name)
		}
	}
}
