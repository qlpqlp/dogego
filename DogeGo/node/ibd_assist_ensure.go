// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"time"

	"dogego/chain"
)

// enrichAssistDiagnostics adds block-assist pool/session fields to IBD progress snapshots.
func enrichAssistDiagnostics(snap map[string]interface{}, pool *BlockAssistCandidates, reg *AssistPeerRegistry) {
	if snap == nil {
		return
	}
	poolN := 0
	if pool != nil {
		poolN = pool.Len()
	}
	snap["assist_peer_pool"] = poolN
	sessN := 0
	if reg != nil {
		sessN = reg.Count()
	}
	snap["assist_active_sessions"] = sessN
	snap["block_assist_workers_started"] = BlockAssistWorkersActive()
}

// MaybeEnsureBlockAssistWorkers starts assist workers once when the pool has peers.
func MaybeEnsureBlockAssistWorkers(p BlockAssistLaunchParams) {
	if p.BlockStore == nil || p.Raw == nil || p.Candidates == nil || p.Candidates.Len() == 0 {
		return
	}
	if !p.Raw.bodiesDownloadActive(p.BlockStore) {
		return
	}
	// Do not resetBlockAssistLaunch while workers are alive: that spawned a second
	// set of goroutines on the same lane IDs and cancelled in-flight getdata.
	if !BlockAssistWorkersActive() {
		EnsureBlockAssistWorkers(p)
	}
}

// maybeEnsureBlockAssistDuringNoPrimary keeps body IBD alive while the primary P2P session is paused.
func maybeEnsureBlockAssistDuringNoPrimary(
	ctx context.Context,
	p chain.Params,
	bs *BlockStoreCtx,
	raw *progressiveRawState,
	candidates **BlockAssistCandidates,
	lastPoolRefresh *time.Time,
	feed *PeerDiscoveryFeed,
	discovered []string,
	pm *PeerMgr,
	scorer *BlockPeerScorer,
	added []string,
	launch func() BlockAssistLaunchParams,
	refreshDiscovery func() []string,
) {
	if bs == nil || raw == nil || !raw.bodiesDownloadActive(bs) || launch == nil {
		return
	}
	if *candidates == nil {
		if refreshDiscovery != nil {
			if fresh := refreshDiscovery(); len(fresh) > 0 {
				discovered = fresh
			}
		}
		*candidates = seedBlockAssistCandidates(ctx, p, bs, scorer, feed, discovered)
	} else if lastPoolRefresh != nil && time.Since(*lastPoolRefresh) >= blockAssistCandidatesRefreshInterval {
		RefreshBlockAssistPool(*candidates, DiscoverySnapshot(feed, discovered), pm, scorer, bs, added)
		*lastPoolRefresh = time.Now()
	}
	MaybeEnsureBlockAssistWorkers(launch())
}
