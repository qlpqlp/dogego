// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"time"

	"dogego/applog"
)

// ibdThroughputTargetBPM is the stored/min floor during deep body IBD (~2k operator target).
const ibdThroughputTargetBPM = 2000

// ibdThroughputBoostInterval throttles boost actions so we do not churn peers every tick.
const ibdThroughputBoostInterval = 15 * time.Second

// ibdThroughputGovernorTick is the background poll while bodies lag headers.
const ibdThroughputGovernorTick = 10 * time.Second

// ibdShortContigRateWindow reacts to recent hole-fill stalls (10m rolling avg hides burst-then-freeze).
const ibdShortContigRateWindow = 90 * time.Second

// ibdHoleBoostStallAfter releases the contiguous hole faster while throughput is below target.
const ibdHoleBoostStallAfter = 12 * time.Second

// ibdThroughputBoostDuration keeps boost levers active briefly after each recovery pass.
const ibdThroughputBoostDuration = 60 * time.Second

// ibdBoostMaxFrontierClaimPeers races more peers on the hole during low-throughput recovery.
const ibdBoostMaxFrontierClaimPeers = 8

// ibdBoostLaneBudget caps per-peer in-flight while stored/min is below target.
const ibdBoostLaneBudget = 32

// ibdBoostMaxAheadInFlight caps ahead heights past the hole per lane during boost trims.
const ibdBoostMaxAheadInFlight = 32

// ibdBoostGlobalAheadCap triggers lane trims when total ahead in-flight exceeds this during boost.
const ibdBoostGlobalAheadCap = 384

// IBDThroughputGovernorParams wires the background stored/min governor (assist-only path included).
type IBDThroughputGovernorParams struct {
	Ctx              context.Context
	BlockStore       *BlockStoreCtx
	Raw              *progressiveRawState
	PeerMgr          *PeerMgr
	Assist           **BlockAssistCandidates
	Feed             *PeerDiscoveryFeed
	Discovered       *[]string
	Scorer           *BlockPeerScorer
	Added            func() []string
	Launch           func() BlockAssistLaunchParams
	RefreshDiscovery func() []string
	EnsureAssist     func()
	PrimaryMW        func() *MsgWriter
}

// StartIBDThroughputGovernor runs boost/recovery on a timer independent of primary P2P read stalls.
func StartIBDThroughputGovernor(p IBDThroughputGovernorParams) {
	if p.Ctx == nil || p.BlockStore == nil || p.Raw == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(ibdThroughputGovernorTick)
		defer ticker.Stop()
		var lastBoost time.Time
		var lastRecover time.Time
		var lastAssistRefresh time.Time
		for {
			select {
			case <-p.Ctx.Done():
				return
			case <-ticker.C:
				if !p.Raw.bodiesDownloadActive(p.BlockStore) {
					continue
				}
				assist := peerAssistPool(p.Assist)
				discovered := peerDiscoveredAddrs(p.Discovered)
				added := []string(nil)
				if p.Added != nil {
					added = p.Added()
				}
				var mw *MsgWriter
				if p.PrimaryMW != nil {
					mw = p.PrimaryMW()
				}
				MaybeBoostIBDThroughput(mw, p.PeerMgr, p.Raw, p.BlockStore, assist, p.Feed, discovered, p.Scorer, added, &lastBoost, &lastAssistRefresh, p.Launch, p.RefreshDiscovery)
				MaybeRecoverIBDStall(mw, p.PeerMgr, p.Raw, p.BlockStore, assist, p.Feed, discovered, p.Scorer, added, &lastRecover, p.Launch, p.RefreshDiscovery, p.EnsureAssist)
			}
		}
	}()
}

func peerAssistPool(p **BlockAssistCandidates) *BlockAssistCandidates {
	if p == nil || *p == nil {
		return nil
	}
	return *p
}

func peerDiscoveredAddrs(p *[]string) []string {
	if p == nil || *p == nil {
		return nil
	}
	return *p
}

// recentShortContiguousBlocksPerMinuteLocked is contiguous blk/min over ~90s. Caller holds s.mu.
func (s *progressiveRawState) recentShortContiguousBlocksPerMinuteLocked() float64 {
	if s == nil || len(s.contigRateSamples) < 2 {
		return 0
	}
	now := time.Now()
	first := s.contigRateSamples[0]
	last := s.contigRateSamples[len(s.contigRateSamples)-1]
	cutoff := now.Add(-ibdShortContigRateWindow)
	for i := range s.contigRateSamples {
		if !s.contigRateSamples[i].at.Before(cutoff) {
			first = s.contigRateSamples[i]
			break
		}
	}
	elapsed := last.at.Sub(first.at)
	if elapsed < ibdRateMinWindow {
		return 0
	}
	delta := last.cum - first.cum
	if delta <= 0 {
		return 0
	}
	return float64(delta) / elapsed.Minutes()
}

func (s *progressiveRawState) throughputBoostActiveLocked(bs *BlockStoreCtx) bool {
	if s == nil {
		return false
	}
	if !s.throughputBoostUntil.IsZero() && time.Now().Before(s.throughputBoostUntil) {
		return true
	}
	return s.shouldSeekThroughputBoostLocked(bs)
}

// throughputBoostActive reports whether low-throughput recovery levers should apply.
func (s *progressiveRawState) throughputBoostActive(bs *BlockStoreCtx) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.throughputBoostActiveLocked(bs)
}

func (s *progressiveRawState) shouldSeekThroughputBoostLocked(bs *BlockStoreCtx) bool {
	if bs == nil || !BodiesBehindHeaders(bs) {
		return false
	}
	if bs.Journal == nil {
		return false
	}
	tip, err := bs.Journal.TipHeight()
	if err != nil || tip < 0 {
		return false
	}
	cont := bs.ContiguousRawHeight()
	if cont < 0 {
		cont = 0
	}
	if tip-cont <= forwardIBDParallelWindow {
		return false
	}
	short := s.recentShortContiguousBlocksPerMinuteLocked()
	if short >= ibdThroughputTargetBPM {
		return false
	}
	if short > 0 {
		return true
	}
	frontier := cont + 1
	if _, blocked := s.inFlight[frontier]; blocked && !s.stallingSince.IsZero() {
		if time.Since(s.stallingSince) >= 8*time.Second {
			return true
		}
	}
	long := s.recentContiguousBlocksPerMinuteLocked()
	if long > 0 && long < ibdThroughputTargetBPM {
		return true
	}
	// Large ahead pipeline with a blocked hole — stored/min will collapse even if the
	// rolling average has not caught up yet.
	var ahead int64
	for h := range s.inFlight {
		if h > frontier {
			ahead++
		}
	}
	return ahead >= ibdBoostGlobalAheadCap
}

func (s *progressiveRawState) maxFrontierClaimPeersLocked(bs *BlockStoreCtx) int {
	if s.throughputBoostActiveLocked(bs) {
		return ibdBoostMaxFrontierClaimPeers
	}
	return maxFrontierClaimPeers
}

// trimLaneAheadOfFrontierLocked drops far-ahead claims on one lane so bandwidth serves the hole.
func (s *progressiveRawState) trimLaneAheadOfFrontierLocked(lane int, frontier int64, maxAhead int) int {
	if s == nil || lane < 0 || maxAhead < 0 || s.inFlightLane == nil {
		return 0
	}
	capHi := frontier + int64(maxAhead)
	freed := 0
	for h, l := range s.inFlightLane {
		if l != lane || h <= frontier {
			continue
		}
		if h > capHi {
			delete(s.inFlight, h)
			delete(s.inFlightLane, h)
			s.clearFrontierClaimsLocked(h)
			freed++
		}
	}
	if freed > 0 {
		s.idleFull = false
	}
	return freed
}

func (s *progressiveRawState) rebalanceAheadForHoleLocked(bs *BlockStoreCtx) int {
	if bs == nil {
		return 0
	}
	cont := bs.ContiguousRawHeight()
	if cont < 0 {
		return 0
	}
	frontier := cont + 1
	if !s.throughputBoostActiveLocked(bs) {
		return 0
	}
	totalFreed := 0
	for lane := range s.laneAddr {
		totalFreed += s.trimLaneAheadOfFrontierLocked(lane, frontier, ibdBoostMaxAheadInFlight)
	}
	workers := s.syncWorkers
	if workers < 1 {
		workers = 1
	}
	for lane := 0; lane < workers; lane++ {
		totalFreed += s.trimLaneAheadOfFrontierLocked(lane, frontier, ibdBoostMaxAheadInFlight)
	}
	return totalFreed
}

func (s *progressiveRawState) applyThroughputBoostLocked(bs *BlockStoreCtx) int {
	cap := ibdBoostLaneBudget
	if s.laneBudgetApplied == nil {
		s.laneBudgetApplied = make(map[int]int)
	}
	workers := s.syncWorkers
	if workers < 1 {
		workers = 1
	}
	totalFreed := 0
	trimLane := func(lane int) {
		s.laneBudgetApplied[lane] = cap
		totalFreed += s.trimLaneInFlightToBudgetLocked(lane, cap)
		totalFreed += s.trimLaneAheadOfFrontierLocked(lane, bs.ContiguousRawHeight()+1, ibdBoostMaxAheadInFlight)
	}
	for lane := range s.laneAddr {
		trimLane(lane)
	}
	for lane := 0; lane < workers; lane++ {
		trimLane(lane)
	}
	s.idleFull = false
	return totalFreed
}

// MaybeBoostIBDThroughput keeps stored/min above the target during deep body IBD by refreshing
// peers, holding lane budgets, racing the hole, and soft-releasing head-of-line stalls sooner.
func MaybeBoostIBDThroughput(
	mw *MsgWriter,
	pm *PeerMgr,
	raw *progressiveRawState,
	bs *BlockStoreCtx,
	assist *BlockAssistCandidates,
	feed *PeerDiscoveryFeed,
	discovered []string,
	scorer *BlockPeerScorer,
	added []string,
	lastBoost *time.Time,
	lastAssistRefresh *time.Time,
	launch func() BlockAssistLaunchParams,
	refreshDiscovery func() []string,
) {
	if raw == nil || lastBoost == nil || bs == nil || !BodiesBehindHeaders(bs) {
		return
	}
	raw.mu.Lock()
	seek := raw.shouldSeekThroughputBoostLocked(bs)
	raw.mu.Unlock()
	if !seek {
		return
	}
	if !lastBoost.IsZero() && time.Since(*lastBoost) < ibdThroughputBoostInterval {
		return
	}
	*lastBoost = time.Now()

	raw.mu.Lock()
	shortBPM := raw.recentShortContiguousBlocksPerMinuteLocked()
	longBPM := raw.recentContiguousBlocksPerMinuteLocked()
	raw.throughputBoostUntil = time.Now().Add(ibdThroughputBoostDuration)
	freed := raw.applyThroughputBoostLocked(bs)
	raw.mu.Unlock()

	raw.ensureBodyDownloadArmed(bs)
	raw.realignProbeToLowestMissing(bs)

	var book *AddrBook
	if pm != nil {
		book = addrBookFromPeerMgr(pm)
	}
	if scorer != nil {
		if peer, stalled := raw.maybePenalizeStallingPeer(bs, scorer, book); stalled && peer != "" {
			applog.Line("block", fmt.Sprintf("IBD throughput boost: hole stall recovery on peer %s", peer))
		} else if stalled {
			applog.Line("block", "IBD throughput boost: released contiguous hole for reclaim")
		}
	}

	if refreshDiscovery != nil {
		if fresh := refreshDiscovery(); len(fresh) > 0 {
			discovered = fresh
		}
	}
	RequestGetAddrFromPeers(mw, pm)
	if assist != nil {
		before := assist.Len()
		RefreshBlockAssistPool(assist, DiscoverySnapshot(feed, discovered), pm, scorer, bs, added)
		if lastAssistRefresh != nil {
			*lastAssistRefresh = time.Now()
		}
		after := assist.Len()
		if after != before {
			applog.Line("block", fmt.Sprintf("IBD throughput boost: assist pool %d → %d", before, after))
		}
	}
	if launch != nil {
		MaybeEnsureBlockAssistWorkers(launch())
	}

	msg := fmt.Sprintf("IBD throughput boost: stored/min short=%.0f long=%.0f (target %d); holding budgets and racing hole", shortBPM, longBPM, ibdThroughputTargetBPM)
	if freed > 0 {
		msg += fmt.Sprintf("; trimmed %d far-ahead claim(s)", freed)
	}
	applog.Line("block", msg)
}
