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

// Deep body IBD uses a longer hole stall. With fat getdata (up to 1000) a peer may still be
// delivering later heights while the contiguous+1 inv is mid-queue; a hard 2s disconnect was
// releasing entire lanes (~800+ claims), collapsing throughput from thousands to ~100 blk/min.
const blockStallingTimeoutBodyIBD = 15 * time.Second
const blockStallingTimeoutBodyIBDEarly = 15 * time.Second

func blockStallingTimeoutFor(bs *BlockStoreCtx) time.Duration {
	if bs != nil && (ShouldDeferConnectForBodyDownload(bs) || ShouldPauseHeaderCatchUpForBodyIBD(bs, 0)) {
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

// maybePenalizeStallingPeer applies Core-style stalling detection: if the contiguous frontier
// height stays claimed in-flight without delivery for the stall timeout, free the hole so
// another peer can fetch it. Deep body IBD soft-releases the frontier first (keeps ahead
// claims) and only hard-disconnects when the same hole stalls again or the lane is idle.
func (s *progressiveRawState) maybePenalizeStallingPeer(bs *BlockStoreCtx, scorer *BlockPeerScorer, book *AddrBook) (peer string, stalled bool) {
	if s == nil || bs == nil || scorer == nil {
		return "", false
	}
	s.mu.Lock()
	if len(s.inFlight) == 0 {
		s.stallingSince = time.Time{}
		s.softStallFrontier = -1
		s.mu.Unlock()
		return "", false
	}
	cont := bs.ContiguousRawHeight()
	frontier := cont + 1
	if cont < 0 {
		frontier = 0
	}
	if _, blocked := s.inFlight[frontier]; !blocked {
		s.stallingSince = time.Time{}
		// Keep softStallFrontier until the hole actually advances, otherwise every
		// soft-release clears the marker and we soft-stall forever without disconnecting.
		if s.softStallFrontier >= 0 && (cont < 0 || cont >= s.softStallFrontier) {
			s.softStallFrontier = -1
		}
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
	soft := deepIBD && laneBusy && s.softStallFrontier != frontier
	if soft {
		delete(s.inFlight, frontier)
		delete(s.inFlightLane, frontier)
		s.softStallFrontier = frontier
		s.stallingSince = time.Time{}
		s.idleFull = false
		s.lastStallPeer = peerAddr
		s.lastStallAt = now
		s.mu.Unlock()
		applog.Line("block", fmt.Sprintf("block download soft-stall: released frontier height %s (lane %d still delivering); kept ahead claims", formatInt64(frontier), lane))
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
			freed++
		}
	} else {
		delete(s.inFlight, frontier)
		freed = 1
	}
	delete(s.laneDownloadSince, lane)
	s.stallingSince = time.Time{}
	s.softStallFrontier = -1
	s.idleFull = false
	s.lastStallPeer = peerAddr
	s.lastStallAt = now
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
