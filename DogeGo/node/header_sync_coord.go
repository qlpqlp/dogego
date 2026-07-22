// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"time"

	"dogego/wire"
)

// RecentHeaderSyncProgress reports whether headers advanced recently (NoteHeadersAppended / dedicated).
func RecentHeaderSyncProgress(within time.Duration) bool {
	syncActivity.mu.Lock()
	lastAt := syncActivity.lastProgressAt
	lastKind := syncActivity.lastKind
	syncActivity.mu.Unlock()
	if lastAt.IsZero() {
		return false
	}
	if lastKind != "headers" && lastKind != "recovery" {
		return false
	}
	return time.Since(lastAt) < within
}

// ShouldDeferBackgroundHeaderSync is true while the dedicated header peer is running and
// making progress - Core uses one coordinated header path during IBD, not parallel loops.
func ShouldDeferBackgroundHeaderSync() bool {
	if dedicatedHeaderSyncRunning.Load() == 0 {
		return false
	}
	// Yield to background recovery when the dedicated link stops advancing headers (rewind/truncate
	// can block for minutes if recovery re-enters on the same goroutine).
	if !RecentHeaderSyncProgress(headerSyncStallEarlyIBD) {
		return false
	}
	return true
}

// NetworkPeerStartHeight returns the highest start height seen on connected peers (Core: best outbound).
func NetworkPeerStartHeight(handshake *wire.DecodedVersion, peerMgr *PeerMgr) int32 {
	var h int32
	if handshake != nil {
		h = handshake.StartHeight
	}
	if peerMgr != nil {
		if m := peerMgr.MaxPeerStartHeight(); m > h {
			h = m
		}
	}
	return h
}
