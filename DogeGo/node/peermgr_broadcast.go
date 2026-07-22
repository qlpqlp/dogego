// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"dogego/mempool"
	"dogego/wire"
)

// BroadcastFeeFilter sends the current min relay feerate to every connected peer (Core BIP133).
func (pm *PeerMgr) BroadcastFeeFilter(pool *mempool.Pool) {
	if pm == nil || pool == nil {
		return
	}
	body := wire.EncodeFeeFilterPayload(LocalMinRelayFeePerKB(pool))
	pm.mu.Lock()
	writers := make([]*MsgWriter, 0, len(pm.sessions))
	for _, l := range pm.sessions {
		if l.mw != nil {
			writers = append(writers, l.mw)
		}
	}
	pm.mu.Unlock()
	for _, mw := range writers {
		_ = mw.Write("feefilter", body)
	}
}

// BroadcastMempool requests BIP35 mempool inv from all connected peers when block sync is caught up.
func (pm *PeerMgr) BroadcastMempool() {
	if pm == nil {
		return
	}
	pm.mu.Lock()
	writers := make([]*MsgWriter, 0, len(pm.sessions))
	for _, l := range pm.sessions {
		if l.mw != nil {
			writers = append(writers, l.mw)
		}
	}
	pm.mu.Unlock()
	for _, mw := range writers {
		_ = mw.Write("mempool", nil)
	}
}
