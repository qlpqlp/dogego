// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "time"

// appendIBDHoleDiagnostics adds lightweight hole-fill UI fields without O(tip) gap scans.
func appendIBDHoleDiagnostics(snap map[string]interface{}, bs *BlockStoreCtx, raw *progressiveRawState) {
	if snap == nil || bs == nil {
		return
	}
	cont := bs.ContiguousRawHeight()
	if cont < 0 {
		return
	}
	hole := cont + 1
	snap["frontier_hole_height"] = hole
	snap["dogego_frontier_hole_height"] = hole
	if raw == nil {
		return
	}
	raw.mu.Lock()
	defer raw.mu.Unlock()
	var ahead int64
	holeInFlight := false
	for h := range raw.inFlight {
		if h == hole {
			holeInFlight = true
		} else if h > hole {
			ahead++
		}
	}
	if ahead > 0 {
		snap["raw_blocks_in_flight_ahead"] = ahead
		snap["dogego_raw_blocks_in_flight_ahead"] = ahead
	}
	if holeInFlight && !raw.stallingSince.IsZero() {
		sec := int64(time.Since(raw.stallingSince).Seconds())
		if sec > 0 {
			snap["hole_blocked_sec"] = sec
			snap["dogego_hole_blocked_sec"] = sec
		}
	}
}
