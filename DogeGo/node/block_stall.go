// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"dogego/applog"
)

// ErrBlockDownloadStall is returned when Core BLOCK_STALLING_TIMEOUT fires on the frontier height.
var ErrBlockDownloadStall = errors.New("block download stall")

// Core validation.h / net_processing BLOCK_STALLING_TIMEOUT - near tip, disconnect the peer
// holding the next height when the download window cannot advance.
const blockStallingTimeout = 2 * time.Second

// Deep body IBD uses a longer hole stall. With fat getdata (up to 256) a peer may still be
// delivering later heights while the contiguous+1 inv is mid-queue; a hard 2s disconnect was
// releasing entire lanes (~800+ claims), collapsing throughput from thousands to ~100 blk/min.
// Core uses multi-minute BLOCK_STALLING_TIMEOUT; 45s deep / 15s early reduces peer thrash while
// soft-stall still frees the hole for another lane first.
const blockStallingTimeoutBodyIBD = 45 * time.Second
const blockStallingTimeoutBodyIBDEarly = 15 * time.Second

func blockStallingTimeoutFor(bs *BlockStoreCtx) time.Duration {
	if bs != nil && (ShouldDeferConnectForBodyDownload(bs) || ShouldPauseHeaderCatchUpForBodyIBD(bs, 0)) {
		if cont := bs.ContiguousRawHeight(); cont >= 0 && cont < 500_000 {
			return blockStallingTimeoutBodyIBDEarly
		}
		return blockStallingTimeoutBodyIBD
	}
	return blockStallingTimeout
}

func (s *progressiveRawState) noteLanePeer(lane int, peer string) {
	if s == nil || lane < 0 || peer == "" {
		return
	}
	s.mu.Lock()
	if s.laneAddr == nil {
		s.laneAddr = make(map[int]string)
	}
	s.laneAddr[lane] = peer
	s.mu.Unlock()
}

func (s *progressiveRawState) ensureHoleReclaimNotifyLocked() {
	if s.holeReclaimNotify == nil {
		s.holeReclaimNotify = make(chan struct{})
	}
}

// signalHoleReclaimLocked wakes assist/primary pumps so another lane can claim contiguous+1.
// Caller must hold s.mu.
func (s *progressiveRawState) signalHoleReclaimLocked() {
	s.ensureHoleReclaimNotifyLocked()
	select {
	case <-s.holeReclaimNotify:
	default:
		close(s.holeReclaimNotify)
	}
	s.holeReclaimNotify = make(chan struct{})
}

// HoleReclaimWaitCh returns a channel that closes when soft-stall frees the frontier hole.
func (s *progressiveRawState) HoleReclaimWaitCh() <-chan struct{} {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureHoleReclaimNotifyLocked()
	return s.holeReclaimNotify
}

// HoleReclaimPending reports whether the contiguous hole was soft-released and needs reclaim.
func (s *progressiveRawState) HoleReclaimPending() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.softStallFrontier >= 0
}

// clearSoftStallIfAdvancedLocked clears soft-stall markers once contiguous passes the hole.
// Caller must hold s.mu.
func (s *progressiveRawState) clearSoftStallIfAdvancedLocked(cont int64) {
	if s.softStallFrontier < 0 {
		return
	}
	if cont < 0 || cont >= s.softStallFrontier {
		s.softStallFrontier = -1
		s.softStallPeer = ""
		s.softStallCount = 0
	}
}

// maybePenalizeStallingPeer applies Core-style stalling detection: if the contiguous frontier
// height stays claimed in-flight without delivery for the stall timeout, free the hole so
// another peer can fetch it. Deep body IBD soft-releases the frontier first (keeps ahead
// claims) and only hard-disconnects when the same hole stalls again, soft-stall escalates,
// or the lane is idle.
func (s *progressiveRawState) maybePenalizeStallingPeer(bs *BlockStoreCtx, scorer *BlockPeerScorer, book *AddrBook) (peer string, stalled bool) {
	if s == nil || bs == nil || scorer == nil {
		return "", false
	}
	s.mu.Lock()
	if len(s.inFlight) == 0 {
		s.stallingSince = time.Time{}
		s.softStallFrontier = -1
		s.softStallPeer = ""
		s.softStallCount = 0
		s.mu.Unlock()
		return "", false
	}
	cont := bs.ContiguousRawHeight()
	frontier := cont + 1
	if cont < 0 {
		frontier = 0
	}
	s.clearSoftStallIfAdvancedLocked(cont)
	if _, blocked := s.inFlight[frontier]; !blocked {
		s.stallingSince = time.Time{}
		s.mu.Unlock()
		return "", false
	}
	now := time.Now()
	if s.stallingSince.IsZero() {
		s.stallingSince = now
		s.mu.Unlock()
		return "", false
	}
	stallTO := blockStallingTimeoutFor(bs)
	if s.throughputBoostActiveLocked(bs) && stallTO > ibdHoleBoostStallAfter {
		stallTO = ibdHoleBoostStallAfter
	}
	if now.Sub(s.stallingSince) < stallTO {
		s.mu.Unlock()
		return "", false
	}
	lane := 0
	if s.inFlightLane != nil {
		if l, ok := s.inFlightLane[frontier]; ok {
			lane = l
		}
	}
	peerAddr := ""
	if s.laneAddr != nil {
		peerAddr = s.laneAddr[lane]
	}
	deepIBD := bs != nil && (ShouldDeferConnectForBodyDownload(bs) || ShouldPauseHeaderCatchUpForBodyIBD(bs, 0))
	laneBusy := false
	if s.laneDownloadSince != nil {
		if since, ok := s.laneDownloadSince[lane]; ok && now.Sub(since) < stallTO {
			laneBusy = true
		}
	}
	// Soft stall: peer is still delivering ahead heights — free only the hole so another
	// lane can claim contiguous+1 without nuking hundreds of in-flight bodies.
	// Do NOT floor that peer's budget (lastStallPeer); soft-stall is HOL, not a slow peer.
	sameHoleSofts := 0
	if s.softStallFrontier == frontier {
		sameHoleSofts = s.softStallCount
	}
	boost := s.throughputBoostActiveLocked(bs)
	soft := deepIBD && sameHoleSofts < softStallEscalateCount && (laneBusy || boost)
	if soft {
		delete(s.inFlight, frontier)
		delete(s.inFlightLane, frontier)
		s.clearFrontierClaimsLocked(frontier)
		if s.softStallFrontier == frontier {
			s.softStallCount++
		} else {
			s.softStallFrontier = frontier
			s.softStallCount = 1
		}
		s.softStallPeer = peerAddr
		s.stallingSince = time.Time{}
		s.idleFull = false
		s.signalHoleReclaimLocked()
		count := s.softStallCount
		trimmed := s.trimLaneAheadOfFrontierLocked(lane, frontier, ibdBoostMaxAheadInFlight)
		s.mu.Unlock()
		applog.Line("block", fmt.Sprintf("block download soft-stall: released frontier height %s (lane %d still delivering, soft %d/%d); kept ahead claims", formatInt64(frontier), lane, count, softStallEscalateCount))
		if trimmed > 0 {
			applog.Line("block", fmt.Sprintf("block download soft-stall: trimmed %d far-ahead height(s) on lane %d", trimmed, lane))
		}
		return "", true
	}
	if slot := s.activeBatch[lane]; slot != nil && slot.cancel != nil {
		slot.cancel()
		delete(s.activeBatch, lane)
	}
	freed := 0
	if s.inFlightLane != nil {
		for h, l := range s.inFlightLane {
			if l != lane {
				continue
			}
			delete(s.inFlight, h)
			delete(s.inFlightLane, h)
			s.clearFrontierClaimsLocked(h)
			freed++
		}
	} else {
		delete(s.inFlight, frontier)
		s.clearFrontierClaimsLocked(frontier)
		freed = 1
	}
	delete(s.laneDownloadSince, lane)
	if s.laneDelivery != nil {
		delete(s.laneDelivery, lane)
	}
	if s.laneBudgetApplied != nil {
		delete(s.laneBudgetApplied, lane)
	}
	s.stallingSince = time.Time{}
	s.softStallFrontier = -1
	s.softStallPeer = ""
	s.softStallCount = 0
	s.idleFull = false
	s.lastStallPeer = peerAddr
	s.lastStallAt = now
	if s.laneBudgetProbeUntil == nil {
		s.laneBudgetProbeUntil = make(map[int]time.Time)
	}
	s.laneBudgetProbeUntil[lane] = now.Add(2 * ibdPeerDeliveryWindow)
	s.signalHoleReclaimLocked()
	s.mu.Unlock()
	if peerAddr != "" {
		penalizeBlockPeer(scorer, book, peerAddr, true)
		NoteBlockPeerDisconnect(peerAddr, "block stall at height "+formatInt64(frontier))
		applog.Line("block", fmt.Sprintf("block download stall: peer %s held height %s in flight >%s without delivery; released %d in-flight height(s) and disconnecting (Core BLOCK_STALLING_TIMEOUT)", peerAddr, formatInt64(frontier), stallTO, freed))
		return peerAddr, true
	}
	applog.Line("block", "block download stall: released in-flight height "+formatInt64(frontier)+" after "+stallTO.String()+" without delivery")
	return "", true
}

func blockStallError(peer string) error {
	if peer == "" {
		return ErrBlockDownloadStall
	}
	return fmt.Errorf("%w: %s", ErrBlockDownloadStall, peer)
}

func formatInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}
