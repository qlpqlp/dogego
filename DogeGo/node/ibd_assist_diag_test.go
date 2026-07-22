// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestEnrichAssistDiagnosticsAuto(t *testing.T) {
	pool := NewBlockAssistCandidates([]string{"1.2.3.4:22556"}, nil)
	reg := NewAssistPeerRegistry()
	var current *BlockAssistCandidates
	SetIBDAssistDiagnostics(func() *BlockAssistCandidates { return current }, reg)
	current = pool
	snap := map[string]interface{}{}
	enrichAssistDiagnosticsAuto(snap)
	if snap["assist_peer_pool"] != 1 {
		t.Fatalf("pool=%v want 1", snap["assist_peer_pool"])
	}
	if snap["block_assist_workers_started"] != false {
		t.Fatalf("started=%v", snap["block_assist_workers_started"])
	}
}
