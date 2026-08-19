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

// Core validation.h / net_processing BLOCK_STALLING_TIMEOUT - disconnect the peer holding
// the next height when the download window cannot advance.
const blockStallingTimeout = 2 * time.Second

// Aliases kept for snapshot / tests. ltcd's 3-minute maxStallDuration is for a single
// fat-getdata sync peer; mixing that with Core's 2s hole-stall (or stretching to 15s)
// left the contiguous height stuck while later blocks filled the window.
const blockStallingTimeoutBodyIBD = blockStallingTimeout
const blockStallingTimeoutBodyIBDEarly = blockStallingTimeout

func blockStallingTimeoutFor(bs *BlockStoreCtx) time.Duration {
	_ = bs
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
	if len(s.inFlight) == 0 {
		s.stallingSince = time.Time{}
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
	if slot := s.activeBatch[lane]; slot != nil && slot.cancel != nil {
		slot.cancel()
		delete(s.activeBatch, lane)
	}
	peerAddr := ""
	if s.laneAddr != nil {
		peerAddr = s.laneAddr[lane]
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
