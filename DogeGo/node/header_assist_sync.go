// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"

	"dogego/applog"
	"dogego/chain"
	"dogego/store"
)

// maxParallelHeaderAssistPeers is how many extra outbound links run headers-only sync during primary IBD.
const maxParallelHeaderAssistPeers = 2

// earlyChainSingleWriterMaxTip disables parallel header assist below this height. Early mainnet
// retarget windows and fast header batches race on one journal (false compressed-period rewinds and forks).
const earlyChainSingleWriterMaxTip int64 = 100_000

// StartParallelHeaderAssist launches headers-only getheaders loops on spare probed peers (Core-style
// multi-link header download). Cancel the returned func when the primary header session is chosen.
// shouldParallelHeaderAssist is false when headers already cover the chain but bodies lag - extra
// header peers can append conflicting batches during forward block IBD (primary + assist share one journal).
func shouldParallelHeaderAssist(j *store.HeaderJournal, bs *BlockStoreCtx) bool {
	if j == nil {
		return false
	}
	tip, err := j.TipHeight()
	if err != nil || tip < 1 || tip < earlyChainSingleWriterMaxTip {
		return false
	}
	if bs != nil && BodiesBehindHeaders(bs) {
		return false
	}
	return true
}

func StartParallelHeaderAssist(ctx context.Context, peers []headerSyncPeer, skipAddr string, p chain.Params, j *store.HeaderJournal, aux *store.HeaderAuxJournal, bs *BlockStoreCtx, raw *progressiveRawState, feed *PeerDiscoveryFeed, scorer *BlockPeerScorer, book *AddrBook) context.CancelFunc {
	if len(peers) < 2 || j == nil || !shouldParallelHeaderAssist(j, bs) {
		return func() {}
	}
	assistCtx, cancel := context.WithCancel(ctx)
	started := 0
	for _, peer := range peers {
		if started >= maxParallelHeaderAssistPeers {
			break
		}
		if peer.addr == skipAddr || peer.mw == nil {
			continue
		}
		started++
		go runHeaderAssistPeer(assistCtx, peer, p, j, aux, bs, raw, feed, scorer, book)
	}
	if started > 0 {
		applog.Line("headers", fmt.Sprintf("parallel header assist: %d extra peer(s) fetching headers", started))
	}
	return cancel
}

func runHeaderAssistPeer(ctx context.Context, peer headerSyncPeer, p chain.Params, j *store.HeaderJournal, aux *store.HeaderAuxJournal, bs *BlockStoreCtx, raw *progressiveRawState, feed *PeerDiscoveryFeed, scorer *BlockPeerScorer, book *AddrBook) {
	defer closeHeaderSyncPeer(peer)
	err := DownloadHeaders(ctx, peer.mw, p, j, aux, nil, bs, 0, raw, peer.startHeight(), feed, true, scorer, book)
	if err != nil && ctx.Err() == nil {
		noteHeaderSyncPeerFailure(scorer, book, peer.addr, err)
		applog.Line("headers", fmt.Sprintf("header assist %s ended: %v", peer.addr, err))
	}
}
