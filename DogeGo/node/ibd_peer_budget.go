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

// ibdBudgetSteps is the asymmetric climb ladder (Core starts at 16; fat only when earned).
var ibdBudgetSteps = []int{
	ibdPeerInFlightSlowFloor,
	ibdPeerInFlightSlow,
	ibdPeerInFlightInitial,
	32,
	ibdPeerInFlightFast,
	128,
	ibdPeerInFlightMax,
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

// effectiveLaneDownloadTimeoutLocked returns the in-flight window for one lane.
// During body IBD the base cap is 30s; lanes with recent deliveries may use up to 90s
// (still below Core's multi-minute window so disconnects do not freeze claims for long).
func (s *progressiveRawState) effectiveLaneDownloadTimeoutLocked(bs *BlockStoreCtx, lanes, lane int) time.Duration {
	base := EffectiveBlockDownloadTimeout(bs, lanes)
	if base != bodyIBDBlockDownloadTimeout {
		return base
	}
	if s != nil && s.laneDeliveryRateLocked(lane) > 0 {
		others := lanes - 1
		if others < 0 {
			others = 0
		}
		core := BlockDownloadTimeout(others, 60)
		if core > bodyIBDProgressDownloadTimeout {
			return bodyIBDProgressDownloadTimeout
		}
		return core
	}
	return bodyIBDBlockDownloadTimeout
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

func ibdBudgetStepIndex(budget int) int {
	best := 0
	for i, step := range ibdBudgetSteps {
		if budget >= step {
			best = i
		}
	}
	return best
}

func ibdBudgetStepUp(budget int) int {
	i := ibdBudgetStepIndex(budget)
	if i+1 < len(ibdBudgetSteps) {
		return ibdBudgetSteps[i+1]
	}
	return ibdBudgetSteps[len(ibdBudgetSteps)-1]
}

func ibdBudgetStepDown(budget int) int {
	i := ibdBudgetStepIndex(budget)
	if i > 0 {
		return ibdBudgetSteps[i-1]
	}
	return ibdBudgetSteps[0]
}

// targetBudgetFromRate maps delivery rate to a desired in-flight budget.
// Rate alone never floors below Core's 16 - that death spiral (trim→slow rate→budget 4)
// was observed live: dozens of "trimmed … to budget 4" with contiguous stuck.
// Only hard stall / download-timeout paths may apply SlowFloor / Slow.
// During deep IBD with small/mid Dogecoin bodies, start above 16 so multi-peer pipes
// stay full (live: 25 lanes × ~10 in flight ≈ Core with one peer; scraps kept rates
// too low to ever climb the adaptive ladder).
func targetBudgetFromRate(rate float64, samples int) int {
	if samples == 0 || rate <= 0 {
		return ibdPeerInFlightInitial
	}
	switch {
	case rate < 2:
		return ibdPeerInFlightInitial
	case rate < 4:
		return 32
	case rate < 8:
		return ibdPeerInFlightFast
	case rate < 12:
		return 128
	case rate < 20:
		return 128
	default:
		return ibdPeerInFlightMax
	}
}

// ibdDeepBodyStartBudget is the adaptive floor while download-first IBD still has
// relatively small bodies. Core's 16 under-fills dozens of Dogecoin assist peers.
func ibdDeepBodyStartBudget(bs *BlockStoreCtx) int {
	if bs == nil || !ShouldDeferConnectForBodyDownload(bs) {
		return ibdPeerInFlightInitial
	}
	h := bs.ContiguousRawHeight()
	switch {
	case h < 1_000_000:
		return 128
	case h < 3_000_000:
		return ibdPeerInFlightFast // 64
	default:
		return 32
	}
}

// estimateBodyBytesForHeight approximates Dogecoin body size for byte-in-flight caps.
func estimateBodyBytesForHeight(height int64) int64 {
	switch {
	case height < 100_000:
		return 500
	case height < 500_000:
		return 2_000
	case height < 1_000_000:
		return 10_000
	case height < 3_000_000:
		return 30_000
	default:
		return 60_000
	}
}

func capBudgetByBytes(budget int, heightHint int64) int {
	if budget < 1 {
		return budget
	}
	est := estimateBodyBytesForHeight(heightHint)
	if est < 1 {
		return budget
	}
	maxByBytes := int(ibdPeerByteCap / est)
	if maxByBytes < ibdPeerInFlightInitial {
		maxByBytes = ibdPeerInFlightInitial
	}
	if budget > maxByBytes {
		return maxByBytes
	}
	return budget
}

// peerInFlightBudget returns how many heights this sync lane may hold in flight.
// Unknown peers start at 16; hard-stall/timeout peers drop briefly; fast peers climb
// one budget step at a time so soft-stalls cannot permanently collapse throughput.
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
	// Low stored/min recovery: cap every lane so getdata refill cannot instantly rebuild
	// a 900+ block ahead pipeline (live: boost trimmed 374 claims then peers refilled +128).
	if s.throughputBoostActiveLocked(bs) {
		cap := ibdBoostLaneBudget
		if s.softStallPeer != "" && s.laneAddr != nil {
			if addr := s.laneAddr[lane]; addr != "" && addr == s.softStallPeer {
				cap = ibdPeerInFlightSlow
			}
		}
		s.setLaneBudgetAppliedLocked(lane, cap)
		return cap
	}
	now := time.Now()
	// Hard stall only floors the budget (soft-stall uses softStallPeer for hole denial).
	if s.lastStallPeer != "" && s.laneAddr != nil {
		if addr := s.laneAddr[lane]; addr != "" && addr == s.lastStallPeer {
			if !s.lastStallAt.IsZero() && now.Sub(s.lastStallAt) < 2*ibdPeerDeliveryWindow {
				s.setLaneBudgetAppliedLocked(lane, ibdPeerInFlightSlowFloor)
				return ibdPeerInFlightSlowFloor
			}
		}
	}
	if s.lastDownloadTimeoutPeer != "" && s.laneAddr != nil {
		if addr := s.laneAddr[lane]; addr != "" && addr == s.lastDownloadTimeoutPeer {
			if !s.lastDownloadTimeoutAt.IsZero() && now.Sub(s.lastDownloadTimeoutAt) < 2*ibdPeerDeliveryWindow {
				s.setLaneBudgetAppliedLocked(lane, ibdPeerInFlightSlow)
				return ibdPeerInFlightSlow
			}
		}
	}

	rate := s.laneDeliveryRateLocked(lane)
	samples := 0
	if s.laneDelivery != nil {
		samples = len(s.laneDelivery[lane])
	}
	floor := ibdDeepBodyStartBudget(bs)
	target := targetBudgetFromRate(rate, samples)
	if target < floor {
		target = floor
	}
	// No samples yet: probe at the deep-IBD floor so first getdata is fat.
	if samples == 0 {
		target = floor
	}

	applied := floor
	if s.laneBudgetApplied != nil {
		if prev, ok := s.laneBudgetApplied[lane]; ok && prev > 0 {
			applied = prev
		}
	}
	// Outside an active hard-floor window, never keep applied below the deep-IBD floor.
	if applied < floor {
		applied = floor
	}
	// After hard penalty expires, re-probe at least the floor so floored peers climb again.
	if s.laneBudgetProbeUntil != nil {
		if until, ok := s.laneBudgetProbeUntil[lane]; ok {
			if now.Before(until) && applied < floor {
				applied = floor
			}
			if !now.Before(until) {
				delete(s.laneBudgetProbeUntil, lane)
				if applied < floor {
					applied = floor
				}
			}
		}
	}

	if target > applied {
		applied = ibdBudgetStepUp(applied)
		if applied > target {
			applied = target
		}
	} else if target < applied {
		// During low-throughput recovery, hold budgets so soft-stall / hole cycles
		// cannot step every lane down and collapse stored/min to hundreds blk/min.
		if !s.throughputBoostActiveLocked(bs) {
			applied = ibdBudgetStepDown(applied)
			if applied < target {
				applied = target
			}
			if applied < floor {
				applied = floor
			}
		}
	}

	heightHint := int64(0)
	if bs != nil {
		heightHint = bs.ContiguousRawHeight()
	}
	applied = capBudgetByBytes(applied, heightHint)

	// Many parallel peers: prefer Core-like moderate in-flight once bodies grow.
	workers := s.syncWorkers
	if workers >= ibdManyPeersWorkerFloor && estimateBodyBytesForHeight(heightHint) >= 10_000 && applied > ibdManyPeersBudgetCap {
		applied = ibdManyPeersBudgetCap
	}

	s.setLaneBudgetAppliedLocked(lane, applied)
	return applied
}

func (s *progressiveRawState) setLaneBudgetAppliedLocked(lane, budget int) {
	if s.laneBudgetApplied == nil {
		s.laneBudgetApplied = make(map[int]int)
	}
	s.laneBudgetApplied[lane] = budget
}

// trimLaneInFlightToBudgetLocked releases excess ahead claims when a lane's budget drops.
// Keeps the lowest heights (including the hole if owned). Returns freed count.
// Caller must hold s.mu.
func (s *progressiveRawState) trimLaneInFlightToBudgetLocked(lane, budget int) int {
	if s == nil || lane < 0 || budget < 0 || s.inFlightLane == nil {
		return 0
	}
	var heights []int64
	for h, l := range s.inFlightLane {
		if l == lane {
			heights = append(heights, h)
		}
	}
	if len(heights) <= budget {
		return 0
	}
	// Sort ascending so we drop the highest (ahead) first.
	for i := 0; i < len(heights); i++ {
		for j := i + 1; j < len(heights); j++ {
			if heights[j] < heights[i] {
				heights[i], heights[j] = heights[j], heights[i]
			}
		}
	}
	freed := 0
	for i := budget; i < len(heights); i++ {
		h := heights[i]
		delete(s.inFlight, h)
		delete(s.inFlightLane, h)
		freed++
	}
	if freed > 0 {
		s.idleFull = false
	}
	return freed
}

// laneBudgetSnapshotLocked builds peer→budget for RPC/UI without mutating ramp state.
// Caller must hold s.mu.
func (s *progressiveRawState) laneBudgetSnapshotLocked(bs *BlockStoreCtx) map[string]int {
	out := make(map[string]int)
	if s == nil || s.laneAddr == nil {
		return out
	}
	now := time.Now()
	for lane, addr := range s.laneAddr {
		if addr == "" {
			continue
		}
		budget := ibdPeerInFlightInitial
		if bs != nil {
			budget = ibdDeepBodyStartBudget(bs)
		} else if s.lastTip > 100_000 || s.blocksStoredIBD > 10_000 {
			// snapshot() often has no BlockStoreCtx; still show the deep-IBD floor.
			budget = ibdPeerInFlightFast
		}
		if s.laneBudgetApplied != nil {
			if prev, ok := s.laneBudgetApplied[lane]; ok && prev > 0 {
				budget = prev
			}
		}
		// Reflect active hard floors without stepping the asymmetric ramp.
		if s.lastStallPeer != "" && addr == s.lastStallPeer &&
			!s.lastStallAt.IsZero() && now.Sub(s.lastStallAt) < 2*ibdPeerDeliveryWindow {
			budget = ibdPeerInFlightSlowFloor
		} else if s.lastDownloadTimeoutPeer != "" && addr == s.lastDownloadTimeoutPeer &&
			!s.lastDownloadTimeoutAt.IsZero() && now.Sub(s.lastDownloadTimeoutAt) < 2*ibdPeerDeliveryWindow {
			budget = ibdPeerInFlightSlow
		} else {
			floor := ibdPeerInFlightInitial
			if bs != nil {
				floor = ibdDeepBodyStartBudget(bs)
			} else if s.lastTip > 100_000 || s.blocksStoredIBD > 10_000 {
				floor = ibdPeerInFlightFast
			}
			if budget < floor {
				budget = floor
			}
		}
		out[addr] = budget
	}
	return out
}
