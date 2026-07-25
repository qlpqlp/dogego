// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"

	"dogego/applog"
	"dogego/p2p"
)

// probeFeeler dials one address from the pool to verify reachability without keeping a relay slot (addrman feeler-lite).
func (pm *PeerMgr) probeFeeler(ctx context.Context) {
	if pm == nil {
		return
	}
	addr := pm.pickFeelerCandidate()
	if addr == "" {
		return
	}
	if pm.blockScorer != nil {
		if st, ok := pm.blockScorer.Stats(addr); ok && st.InCooldown {
			return
		}
	}
	book := addrBookFromPeerMgr(pm)
	RecordOutboundDialTry(book, addr)
	c, err := pm.dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		if p2p.ObserveDialError(addr, err) {
			applog.Line("net", "IPv6 dials disabled (network unreachable); preferring IPv4 peers")
		}
		RecordOutboundHandshakeResult(book, addr, err)
		if pm.blockScorer != nil {
			pm.blockScorer.NoteDialFailure(addr)
		}
		return
	}
	_, err = Handshake(ctx, c, pm.params, pm.userAgent, pm.advertisedServices())
	_ = c.Close()
	if err != nil {
		RecordOutboundHandshakeResult(book, addr, err)
		if pm.blockScorer != nil {
			pm.blockScorer.NoteDialFailure(addr)
		}
		applog.Line("net", "feeler "+addr+": "+err.Error())
		return
	}
	RecordOutboundHandshakeResult(book, addr, nil)
	pm.NoteAddr(addr)
	applog.Line("net", "feeler probe ok "+addr)
}

func (pm *PeerMgr) pickFeelerCandidate() string {
	pm.mu.Lock()
	primary := pm.primary
	skip := make(map[string]struct{}, len(pm.sessions))
	for a := range pm.sessions {
		skip[a] = struct{}{}
	}
	book := pm.addrs
	pm.mu.Unlock()
	if book == nil {
		return ""
	}
	return book.PickFeeler(skip, primary)
}
