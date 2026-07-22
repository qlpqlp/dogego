// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestEnrichAssistDiagnostics(t *testing.T) {
	pool := NewBlockAssistCandidates([]string{"1.2.3.4:22556", "5.6.7.8:22556"}, nil)
	reg := NewAssistPeerRegistry()
	snap := map[string]interface{}{}
	enrichAssistDiagnostics(snap, pool, reg)
	if snap["assist_peer_pool"] != 2 {
		t.Fatalf("pool=%v want 2", snap["assist_peer_pool"])
	}
	if snap["assist_active_sessions"] != 0 {
		t.Fatalf("sessions=%v want 0", snap["assist_active_sessions"])
	}
	if snap["block_assist_workers_started"] != false {
		t.Fatalf("started=%v want false", snap["block_assist_workers_started"])
	}
}

func TestResetBlockAssistLaunchClearsLatch(t *testing.T) {
	blockAssistLaunchMu.Lock()
	blockAssistActive = true
	blockAssistLaunchMu.Unlock()
	resetBlockAssistLaunch()
	if BlockAssistWorkersActive() {
		t.Fatal("resetBlockAssistLaunch should clear active latch")
	}
}
