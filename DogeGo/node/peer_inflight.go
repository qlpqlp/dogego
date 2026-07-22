// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "sort"

// syncLaneForPeer maps a connected peer address to the block-download lane used for getdata stripes.
func syncLaneForPeer(addr, primaryAddr string, assist []AssistPeerSnapshot, raw *progressiveRawState) int {
	if addr == "" {
		return -1
	}
	if primaryAddr != "" && addr == primaryAddr {
		return 0
	}
	for _, snap := range assist {
		if snap.Addr == addr {
			return snap.Lane
		}
	}
	if raw != nil {
		return raw.laneForAddr(addr)
	}
	return -1
}

func inflightHeightsJSON(raw *progressiveRawState, lane int) []interface{} {
	if raw == nil || lane < 0 {
		return []interface{}{}
	}
	hs := raw.InflightHeightsForLane(lane)
	out := make([]interface{}, len(hs))
	for i, h := range hs {
		out[i] = h
	}
	return out
}

// InflightHeightsForLane returns block heights with an active getdata claim on lane (Core getpeerinfo inflight).
func (s *progressiveRawState) InflightHeightsForLane(lane int) []int64 {
	if s == nil || lane < 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []int64
	for h, w := range s.inFlightLane {
		if w == lane {
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
