// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "sort"

// maxPeerInfoInflightHeights caps getpeerinfo inflight arrays so IBD with fat
// per-lane budgets does not serialize thousands of heights under the peer lock.
const maxPeerInfoInflightHeights = 32

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
	return heightsToJSONCap(hs, maxPeerInfoInflightHeights)
}

func heightsToJSONCap(hs []int64, capN int) []interface{} {
	if len(hs) == 0 {
		return []interface{}{}
	}
	n := len(hs)
	if capN > 0 && n > capN {
		n = capN
	}
	out := make([]interface{}, n)
	for i := 0; i < n; i++ {
		out[i] = hs[i]
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
	return s.inflightHeightsForLaneLocked(lane)
}

func (s *progressiveRawState) inflightHeightsForLaneLocked(lane int) []int64 {
	if s == nil || lane < 0 || s.inFlightLane == nil {
		return nil
	}
	var out []int64
	for h, w := range s.inFlightLane {
		if w == lane {
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// InflightHeightsByLane snapshots all lane→heights under one lock for getpeerinfo.
func (s *progressiveRawState) InflightHeightsByLane() map[int][]int64 {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.inFlightLane) == 0 {
		return nil
	}
	tmp := make(map[int][]int64)
	for h, lane := range s.inFlightLane {
		tmp[lane] = append(tmp[lane], h)
	}
	out := make(map[int][]int64, len(tmp))
	for lane, hs := range tmp {
		sort.Slice(hs, func(i, j int) bool { return hs[i] < hs[j] })
		out[lane] = hs
	}
	return out
}
