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

// Core validation.h BLOCK_STALLING_TIMEOUT - disconnect when download window cannot advance.
const blockStallingTimeout = 2 * time.Second

// During deep body IBD, ancient getdata often exceeds 2s; a hard 2s disconnect churns peers
// and collapses download rate. Soften timeout and prefer soft cooldown while bodies lag headers.
const blockStallingTimeoutBodyIBD = 15 * time.Second

func blockStallingTimeoutFor(bs *BlockStoreCtx) time.Duration {
	if bs != nil && ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
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
// height stays claimed in-flight without delivery for blockStallingTimeout, cooldown that peer.
// The returned peer address is non-empty when a stall was handled.
func (s *progressiveRawState) maybePenalizeStallingPeer(bs *BlockStoreCtx, scorer *BlockPeerScorer, book *AddrBook) (peer string, stalled bool) {
	if s == nil || bs == nil || scorer == nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.inFlight) == 0 {
		s.stallingSince = time.Time{}
		return "", false
	}
	cont := bs.ContiguousRawHeight()
	frontier := cont + 1
	if cont < 0 {
		frontier = 0
	}
	if _, blocked := s.inFlight[frontier]; !blocked {
		s.stallingSince = time.Time{}
		return "", false
	}
	now := time.Now()
	if s.stallingSince.IsZero() {
		s.stallingSince = now
		return "", false
	}
	stallTO := blockStallingTimeoutFor(bs)
	if now.Sub(s.stallingSince) < stallTO {
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
	delete(s.inFlight, frontier)
	if s.inFlightLane != nil {
		delete(s.inFlightLane, frontier)
	}
	s.stallingSince = time.Time{}
	s.idleFull = false
	s.lastStallPeer = peerAddr
	s.lastStallAt = now
	bodyIBD := ShouldPauseHeaderCatchUpForBodyIBD(bs, 0)
	if peerAddr != "" {
		if bodyIBD {
			// Soft: release the frontier claim and brief cooldown — do not disconnect.
			// Hard 2s stalls during ancient getdata were rotating peers and collapsing blk/min.
			penalizeBlockPeer(scorer, book, peerAddr, false)
			applog.Line("block", "block download stall: peer "+peerAddr+" held height "+formatInt64(frontier)+" in flight >"+stallTO.String()+" without delivery; soft release (body IBD, peer kept)")
			return "", false
		}
		penalizeBlockPeer(scorer, book, peerAddr, true)
		NoteBlockPeerDisconnect(peerAddr, "block stall at height "+formatInt64(frontier))
		applog.Line("block", "block download stall: peer "+peerAddr+" held height "+formatInt64(frontier)+" in flight >"+stallTO.String()+" without delivery; disconnecting peer (Core BLOCK_STALLING_TIMEOUT)")
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
