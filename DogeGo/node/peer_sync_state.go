// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strings"
	"time"

	"dogego/wire"
)

func initPeerSyncFromVersion(link *peerLink, dv *wire.DecodedVersion) {
	if link == nil {
		return
	}
	link.bestHeaderHeight = -1
	link.bestHeaderHash = ""
	link.tipUpdatedUnix = 0
	link.commonBlockHeight = -1
	if dv != nil && dv.StartHeight >= 0 {
		link.bestHeaderHeight = int64(dv.StartHeight)
	}
}

// NotePeerHeaders updates the peer's best known header tip after a headers batch.
// tipHash should be the block hash hex of tipHeight when known (empty keeps the previous hash).
func (pm *PeerMgr) NotePeerHeaders(addr string, tipHeight int64, tipHash string) {
	if pm == nil || addr == "" || tipHeight < 0 {
		return
	}
	tipHash = strings.ToLower(strings.TrimSpace(tipHash))
	pm.mu.Lock()
	defer pm.mu.Unlock()
	l := pm.sessions[addr]
	if l == nil {
		return
	}
	if tipHeight > l.bestHeaderHeight {
		l.bestHeaderHeight = tipHeight
		if tipHash != "" {
			l.bestHeaderHash = tipHash
		}
		l.tipUpdatedUnix = time.Now().Unix()
		return
	}
	if tipHeight == l.bestHeaderHeight && tipHash != "" && tipHash != l.bestHeaderHash {
		// Same height, different tip hash → peer is on a competing fork tip.
		l.bestHeaderHash = tipHash
		l.tipUpdatedUnix = time.Now().Unix()
	}
}

// NotePeerBlockAt records block delivery and common block height (Core getpeerinfo synced_blocks).
func (pm *PeerMgr) NotePeerBlockAt(addr string, height int64) {
	pm.NotePeerBlock(addr)
	if pm == nil || addr == "" || height < 0 {
		return
	}
	pm.mu.Lock()
	if l := pm.sessions[addr]; l != nil && height > l.commonBlockHeight {
		l.commonBlockHeight = height
	}
	pm.mu.Unlock()
}

func peerSyncedHeaders(l *peerLink, tipH int64) int64 {
	if l == nil {
		return tipH
	}
	if l.bestHeaderHeight >= 0 {
		return l.bestHeaderHeight
	}
	if l.peer != nil && l.peer.StartHeight >= 0 {
		return int64(l.peer.StartHeight)
	}
	return tipH
}

func peerSyncedBlocks(l *peerLink, fallback int64) int64 {
	if l == nil {
		return fallback
	}
	if l.commonBlockHeight >= 0 {
		return l.commonBlockHeight
	}
	return fallback
}
