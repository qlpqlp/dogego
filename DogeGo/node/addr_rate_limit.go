// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "math"

// Core net_processing.h addr gossip limits.
const (
	maxAddrRatePerSecond       = 0.1
	maxAddrProcessingTokenBucket = 1000.0
	maxAddrToSend              = 1000.0
)

func (l *peerLink) refillAddrTokensLocked(nowMicro int64) {
	if l == nil {
		return
	}
	if l.addrTokenMicros > 0 && nowMicro > l.addrTokenMicros && l.addrTokenBucket < maxAddrProcessingTokenBucket {
		elapsed := nowMicro - l.addrTokenMicros
		inc := float64(elapsed) * maxAddrRatePerSecond / 1e6
		l.addrTokenBucket = math.Min(l.addrTokenBucket+inc, maxAddrProcessingTokenBucket)
	}
	l.addrTokenMicros = nowMicro
}

func (l *peerLink) grantAddrTokens(n float64) {
	if l == nil || n <= 0 {
		return
	}
	l.addrTokenBucket += n
}

// NoteOutboundGetAddr grants extra addr processing tokens after we request addresses (Core version/getaddr path).
func (pm *PeerMgr) NoteOutboundGetAddr(peerAddr string) {
	if pm == nil || peerAddr == "" {
		return
	}
	pm.mu.Lock()
	if l := pm.sessions[peerAddr]; l != nil {
		l.grantAddrTokens(maxAddrToSend)
	}
	pm.mu.Unlock()
}

func (pm *PeerMgr) peerWhitelistedLocked(fromPeer string) bool {
	for _, a := range pm.preferredAddrs {
		if addnodeMatchesSession(a, fromPeer) {
			return true
		}
	}
	return false
}
