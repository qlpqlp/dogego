// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"
	"time"
)

// TestIBDFlakyPeerRecoveryCertification documents stall-recovery timing used when peers
// chatter without delivering blocks (the height-771 class of failure).
func TestIBDFlakyPeerRecoveryCertification(t *testing.T) {
	cases := []struct {
		name     string
		interval time.Duration
		wantMax  time.Duration
	}{
		{"body_pump_tick", bodyIBDPumpInterval, 2 * time.Second},
		{"body_only_stall", ibdStallNoBlockIntervalBodyOnly, 2 * time.Minute},
		{"assist_stall_relaunch", bodyIBDAssistStallRelaunch, 2 * time.Minute},
		{"assist_session_rotate", blockAssistSessionIdleRotate, time.Minute},
		{"genesis_stall", ibdStallNoBlockIntervalGenesis, 3 * time.Minute},
	}
	for _, tc := range cases {
		if tc.interval <= 0 || tc.interval > tc.wantMax {
			t.Fatalf("%s: interval=%v want (0,%v]", tc.name, tc.interval, tc.wantMax)
		}
	}
	if bodyIBDPumpBatchesPerRound < 1 {
		t.Fatal("pump must issue at least one getdata batch per tick")
	}
	if IdleFetchBatchesPerRound(nil) != 2 {
		t.Fatalf("idle fetch default=%d want 2", IdleFetchBatchesPerRound(nil))
	}
}
