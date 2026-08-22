// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "time"

// laneDeliverySample tracks recent block deliveries for adaptive per-peer in-flight budgets.
type laneDeliverySample struct {
	at time.Time
	n  int
}

// noteLaneBlocksDeliveredLocked records n bodies received on lane for EWMA budgeting.
// Caller must hold s.mu.
func (s *progressiveRawState) noteLaneBlocksDeliveredLocked(lane, n int) {
	if s == nil || lane < 0 || n <= 0 {
		return
	}
	if s.laneDelivery == nil {
		s.laneDelivery = make(map[int][]laneDeliverySample)
	}
	now := time.Now()
	s.laneDelivery[lane] = append(s.laneDelivery[lane], laneDeliverySample{at: now, n: n})
	s.trimLaneDeliveryLocked(lane, now)
}

func (s *progressiveRawState) trimLaneDeliveryLocked(lane int, now time.Time) {
	samples := s.laneDelivery[lane]
	if len(samples) == 0 {
		return
	}
	cut := 0
	for cut < len(samples) && now.Sub(samples[cut].at) > ibdPeerDeliveryWindow {
		cut++
	}
	if cut > 0 {
		s.laneDelivery[lane] = append([]laneDeliverySample(nil), samples[cut:]...)
	}
}

// laneDeliveryRateLocked returns blocks/sec delivered on lane over ibdPeerDeliveryWindow.
func (s *progressiveRawState) laneDeliveryRateLocked(lane int) float64 {
	if s == nil || s.laneDelivery == nil {
		return 0
	}
	now := time.Now()
	s.trimLaneDeliveryLocked(lane, now)
	samples := s.laneDelivery[lane]
	if len(samples) == 0 {
		return 0
	}
	total := 0
	for _, sm := range samples {
		total += sm.n
	}
	elapsed := now.Sub(samples[0].at)
	if elapsed < time.Second {
		elapsed = time.Second
	}
	return float64(total) / elapsed.Seconds()
}

// peerInFlightBudget returns how many heights this sync lane may hold in flight.
// Unknown peers start at 16; slow/stalling peers drop to 4–8; fast peers rise to 64–256.
func (s *progressiveRawState) peerInFlightBudget(bs *BlockStoreCtx, lane int) int {
	base := EffectiveProgressiveBatchSizeForIBD(bs, s.syncWorkerCount())
	if s == nil {
		return base
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.peerInFlightBudgetLocked(bs, lane)
}

func (s *progressiveRawState) peerInFlightBudgetLocked(bs *BlockStoreCtx, lane int) int {
	base := EffectiveProgressiveBatchSizeForIBD(bs, s.syncWorkers)
	if base < 1 {
		base = ibdPeerInFlightInitial
	}
	nearTip := bs != nil && !ShouldDeferConnectForBodyDownload(bs) && !ShouldPauseHeaderCatchUpForBodyIBD(bs, 0)
	if nearTip {
		return base
	}
	// Recent stall on this peer → slow floor.
	if s.lastStallPeer != "" && s.laneAddr != nil {
		if addr := s.laneAddr[lane]; addr != "" && addr == s.lastStallPeer {
			if !s.lastStallAt.IsZero() && time.Since(s.lastStallAt) < 2*ibdPeerDeliveryWindow {
				return ibdPeerInFlightSlowFloor
			}
		}
	}
	if s.lastDownloadTimeoutPeer != "" && s.laneAddr != nil {
		if addr := s.laneAddr[lane]; addr != "" && addr == s.lastDownloadTimeoutPeer {
			if !s.lastDownloadTimeoutAt.IsZero() && time.Since(s.lastDownloadTimeoutAt) < 2*ibdPeerDeliveryWindow {
				return ibdPeerInFlightSlow
			}
		}
	}
	rate := s.laneDeliveryRateLocked(lane)
	samples := 0
	if s.laneDelivery != nil {
		samples = len(s.laneDelivery[lane])
	}
	if samples == 0 || rate <= 0 {
		return ibdPeerInFlightInitial
	}
	// ~blk/sec thresholds sized for early Dogecoin bodies on a LAN/WAN mix.
	switch {
	case rate < 0.5:
		return ibdPeerInFlightSlowFloor
	case rate < 2:
		return ibdPeerInFlightSlow
	case rate < 8:
		return ibdPeerInFlightFast
	case rate < 20:
		if ibdPeerInFlightMax < 128 {
			return ibdPeerInFlightMax
		}
		return 128
	default:
		return ibdPeerInFlightMax
	}
}

// laneBudgetSnapshotLocked builds peer→budget for RPC/UI (caller holds s.mu).
func (s *progressiveRawState) laneBudgetSnapshotLocked(bs *BlockStoreCtx) map[string]int {
	out := make(map[string]int)
	if s == nil || s.laneAddr == nil {
		return out
	}
	for lane, addr := range s.laneAddr {
		if addr == "" {
			continue
		}
		out[addr] = s.peerInFlightBudgetLocked(bs, lane)
	}
	return out
}
