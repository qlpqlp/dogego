// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"fmt"
	"time"

	"dogego/applog"
)

// ErrBlockDownloadTimeout is returned when a peer holds an in-flight getdata batch past Core's download window.
var ErrBlockDownloadTimeout = errors.New("block download timeout")

func (s *progressiveRawState) noteBatchDownloadStartLocked(lane int) {
	if s == nil || lane < 0 {
		return
	}
	if s.laneDownloadSince == nil {
		s.laneDownloadSince = make(map[int]time.Time)
	}
	if _, ok := s.laneDownloadSince[lane]; !ok {
		s.laneDownloadSince[lane] = time.Now()
	}
}

func (s *progressiveRawState) clearLaneDownloadIfIdleLocked(lane int) {
	if s == nil || lane < 0 || s.laneDownloadSince == nil {
		return
	}
	for _, l := range s.inFlightLane {
		if l == lane {
			return
		}
	}
	delete(s.laneDownloadSince, lane)
}

// maybePenalizeDownloadTimeout releases in-flight heights and disconnects when a lane exceeds
// Core's BLOCK_DOWNLOAD_TIMEOUT window (net_processing.cpp nDownloadingSince check).
func (s *progressiveRawState) maybePenalizeDownloadTimeout(bs *BlockStoreCtx, scorer *BlockPeerScorer, book *AddrBook) (peer string, timedOut bool) {
	if s == nil || bs == nil || scorer == nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.laneDownloadSince) == 0 {
		return "", false
	}
	lanes := s.syncWorkers
	if lanes < 1 {
		lanes = 1
	}
	limit := EffectiveBlockDownloadTimeout(bs, lanes)
	now := time.Now()
	for lane, since := range s.laneDownloadSince {
		if now.Sub(since) < limit {
			continue
		}
		hasFlight := false
		for _, l := range s.inFlightLane {
			if l == lane {
				hasFlight = true
				break
			}
		}
		if !hasFlight {
			delete(s.laneDownloadSince, lane)
			continue
		}
		peerAddr := ""
		if s.laneAddr != nil {
			peerAddr = s.laneAddr[lane]
		}
		freed := 0
		for h, l := range s.inFlightLane {
			if l != lane {
				continue
			}
			delete(s.inFlight, h)
			delete(s.inFlightLane, h)
			freed++
		}
		delete(s.laneDownloadSince, lane)
		s.stallingSince = time.Time{}
		s.idleFull = false
		s.lastDownloadTimeoutPeer = peerAddr
		s.lastDownloadTimeoutAt = now
		if peerAddr != "" {
			penalizeBlockPeer(scorer, book, peerAddr, true)
			NoteBlockPeerDisconnect(peerAddr, fmt.Sprintf("download timeout (%s)", limit.Round(time.Second)))
			applog.Line("block", fmt.Sprintf("block download timeout: peer %s held %d height(s) in flight >%s without delivery; disconnecting (Core BLOCK_DOWNLOAD_TIMEOUT)", peerAddr, freed, limit.Round(time.Second)))
			return peerAddr, true
		}
		applog.Line("block", fmt.Sprintf("block download timeout: released %d in-flight height(s) on lane %d after %s", freed, lane, limit.Round(time.Second)))
		return "", true
	}
	return "", false
}

func blockDownloadTimeoutError(peer string) error {
	if peer == "" {
		return ErrBlockDownloadTimeout
	}
	return fmt.Errorf("%w: %s", ErrBlockDownloadTimeout, peer)
}
