// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"sync"

	"dogego/applog"
	"dogego/chain"
)

// mempoolSyncState requests peer mempool inventory once after block download catches up (BIP35 / Core).
type mempoolSyncState struct {
	mu   sync.Mutex
	sent bool
}

func (s *mempoolSyncState) reset() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.sent = false
	s.mu.Unlock()
}

func (s *mempoolSyncState) maybeRequest(ctx context.Context, mw *MsgWriter, p chain.Params, bs *BlockStoreCtx, raw *progressiveRawState, pm *PeerMgr) {
	if s == nil || mw == nil || bs == nil || raw == nil {
		return
	}
	if raw.useShortReadDeadline() {
		return
	}
	s.mu.Lock()
	if s.sent {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	select {
	case <-ctx.Done():
		return
	default:
	}
	if err := mw.Write("mempool", nil); err != nil {
		applog.Line("mempool", "mempool request to primary peer: "+err.Error())
		return
	}
	s.mu.Lock()
	s.sent = true
	s.mu.Unlock()
	applog.Line("mempool", "requested primary peer mempool (BIP35)")
	if pm != nil {
		pm.BroadcastMempool()
		applog.Line("mempool", "requested mempool from relay peers (BIP35)")
	}
}
