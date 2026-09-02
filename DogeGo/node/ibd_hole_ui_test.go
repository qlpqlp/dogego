// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestAppendIBDHoleDiagnostics(t *testing.T) {
	bs := &BlockStoreCtx{}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 100
	bs.contiguousMu.Unlock()
	raw := &progressiveRawState{
		inFlight: map[int64][32]byte{
			101: {},
			150: {},
			200: {},
		},
	}
	snap := map[string]interface{}{}
	appendIBDHoleDiagnostics(snap, bs, raw)
	if snap["frontier_hole_height"] != int64(101) {
		t.Fatalf("hole=%v want 101", snap["frontier_hole_height"])
	}
	if snap["dogego_raw_blocks_in_flight_ahead"] != int64(2) {
		t.Fatalf("ahead=%v want 2", snap["dogego_raw_blocks_in_flight_ahead"])
	}
}

func TestSyncActivityHeadlineHoleBlocked(t *testing.T) {
	head, _ := syncActivityHeadline(SyncActivityInput{
		HeaderTip:                 1000,
		ContiguousBodies:          100,
		LowestMissing:             101,
		InFlightBatches:           50,
		BlocksPerMinute:           300,
		ContiguousBlocksPerMinute: 0,
	}, 20, "", "", 0, "", "", "", "", 0)
	if head != "Blocked at height 101" {
		t.Fatalf("headline=%q want blocked", head)
	}
}
