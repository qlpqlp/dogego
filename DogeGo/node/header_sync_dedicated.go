// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"sync/atomic"

	"dogego/applog"
	"dogego/chain"
	"dogego/store"
)

// DedicatedHeaderSyncEnv runs headers-first sync on a dedicated TCP session so the primary
// outbound link can interleave block getdata (Core multiplexes one loop; DogeGo splits links).
type DedicatedHeaderSyncEnv struct {
	Ctx             context.Context
	Params          chain.Params
	Journal         *store.HeaderJournal
	Aux             *store.HeaderAuxJournal
	FeeFilters      *FeeFilterSet
	BlockStore      *BlockStoreCtx
	RawBackfill     int
	RawFill         *progressiveRawState
	DiscoveryFeed   *PeerDiscoveryFeed
	Scorer          *BlockPeerScorer
	AddrBook        *AddrBook
	OnYieldOrDone   func() // optional: header catch-up still needed (after dedicated exits)
	OnCaughtUp      func() // optional: clear catch-up when network headers are caught up
}

var dedicatedHeaderSyncRunning atomic.Int32

// StartDedicatedHeaderSync launches DownloadHeaders on peer in the background. At most one
// dedicated header IBD runs at a time; background HeaderSyncRecovery runs only after yield,
// disconnect, or watchdog stall (Core-style single coordinator during normal IBD).
func StartDedicatedHeaderSync(env DedicatedHeaderSyncEnv, peer headerSyncPeer) bool {
	if peer.conn == nil || peer.mw == nil {
		return false
	}
	if !dedicatedHeaderSyncRunning.CompareAndSwap(0, 1) {
		return false
	}
	go func() {
		var kickBackground bool
		defer func() {
			dedicatedHeaderSyncRunning.Store(0)
			if kickBackground && env.OnYieldOrDone != nil {
				env.OnYieldOrDone()
			}
		}()
		addr := peer.addr
		startH := peer.startHeight()
		applog.Line("headers", fmt.Sprintf("dedicated header sync on %s (start height %d) - primary link free for block bodies", addr, startH))
		NoteDedicatedHeaderSync(addr, fmt.Sprintf("start height %d", startH))
		if env.Journal != nil {
			env.Journal.ReconcileCountCacheFromDisk()
		}
		if env.BlockStore != nil {
			env.BlockStore.SetNetworkTimeSource(nil, peer.dv)
		}
		err := DownloadHeaders(env.Ctx, peer.mw, env.Params, env.Journal, env.Aux, env.FeeFilters, env.BlockStore, env.RawBackfill, env.RawFill, startH, env.DiscoveryFeed, true, env.Scorer, env.AddrBook)
		tip, _ := env.Journal.TipHeight()
		stillBehind := shouldContinueHeaderCatchUpDuringIBD(env.Journal, startH)
		if err != nil {
			noteHeaderSyncPeerFailure(env.Scorer, env.AddrBook, addr, err)
			NoteDedicatedHeaderSyncDone(addr, err)
			if isHeaderSyncYieldForBackground(err) {
				applog.Line("headers", fmt.Sprintf("dedicated header sync yielded on %s (%v)", addr, err))
			} else if shouldAutoRecoverHeaderSync(err) {
				noteHeaderSyncFailure(err)
				applog.Line("headers", fmt.Sprintf("dedicated header sync on %s ended (%v); background header sync will retry", addr, err))
			} else {
				applog.Line("headers", fmt.Sprintf("dedicated header sync on %s failed: %v", addr, err))
			}
			if stillBehind {
				kickBackground = true
			}
			closeHeaderSyncPeer(peer)
			return
		}
		NoteDedicatedHeaderSyncDone(addr, nil)
		applog.Line("headers", fmt.Sprintf("dedicated header sync on %s finished at tip %d (peer height %d)", addr, tip, startH))
		if stillBehind {
			applog.Line("headers", fmt.Sprintf("header tip %d still behind network (peer %s height %d) - background header sync continues", tip, addr, startH))
			kickBackground = true
		} else if env.OnCaughtUp != nil {
			env.OnCaughtUp()
		}
		closeHeaderSyncPeer(peer)
	}()
	return true
}
