// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"

	"dogego/applog"
	"dogego/chain"
	"dogego/store"
	"dogego/wire"
)

const maxRelayHeaderTopUpPeers = 3

// encodeTopUpGetHeaders builds a getheaders payload from the local header journal tip.
func encodeTopUpGetHeaders(j *store.HeaderJournal, p chain.Params) ([]byte, error) {
	var zero [32]byte
	loc, err := j.BuildBlockLocator(101)
	if err != nil {
		return nil, err
	}
	return wire.EncodeGetHeaders(p.ProtocolVersion, loc, zero)
}

// RequestHeadersTopUp sends getheaders on an existing peer link. Replies are applied on that peer's read loop.
func RequestHeadersTopUp(w *MsgWriter, payload []byte) error {
	if w == nil || len(payload) == 0 {
		return nil
	}
	return w.Write("getheaders", payload)
}

// RelayAddrsOrdered returns up to limit connected relay peer addresses, best block-scorer rank first.
func (pm *PeerMgr) RelayAddrsOrdered(limit int) []string {
	if pm == nil || limit <= 0 {
		return nil
	}
	pm.mu.Lock()
	primary := pm.primary
	addrs := make([]string, 0, len(pm.sessions))
	for _, a := range pm.order {
		l, ok := pm.sessions[a]
		if !ok || l.primary || l.mw == nil {
			continue
		}
		addrs = append(addrs, a)
	}
	scorer := pm.blockScorer
	pm.mu.Unlock()
	if scorer != nil && len(addrs) > 1 {
		addrs = scorer.OrderCandidates(addrs, primary)
	}
	if len(addrs) > limit {
		addrs = addrs[:limit]
	}
	return addrs
}

// RequestHeadersTopUpFromRelays asks relay peers for headers (Core: compare tips from several outbound links).
// Header batches are validated (including chain-work reorg rules) on each relay read loop.
func (pm *PeerMgr) RequestHeadersTopUpFromRelays(p chain.Params, j *store.HeaderJournal, limit int) int {
	if pm == nil || j == nil || limit <= 0 {
		return 0
	}
	payload, err := encodeTopUpGetHeaders(j, p)
	if err != nil {
		return 0
	}
	addrs := pm.RelayAddrsOrdered(limit)
	sent := 0
	for _, addr := range addrs {
		mw := pm.LinkByAddr(addr)
		if mw == nil {
			continue
		}
		if err := RequestHeadersTopUp(mw, payload); err != nil {
			applog.Line("headers", fmt.Sprintf("relay header top-up %s: %v", addr, err))
			continue
		}
		sent++
		applog.Line("headers", fmt.Sprintf("header top-up getheaders → relay %s", addr))
	}
	return sent
}
