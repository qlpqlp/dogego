// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/mempool"
	"dogego/rpc"
	"dogego/store"
	"dogego/wire"
)

const soloPeerAttachInterval = 45 * time.Second

// SoloAttachOpts configures periodic attempts to leave solo founder mode via outbound P2P.
type SoloAttachOpts struct {
	Ctx           context.Context
	Dialer        net.Dialer
	Params        chain.Params
	UserAgent      string
	LocalServices  uint64
	Journal        *store.HeaderJournal
	Aux           *store.HeaderAuxJournal
	BlockStore    *BlockStoreCtx
	FeeFilters    *FeeFilterSet
	RawFill       *progressiveRawState
	DiscoveryFeed *PeerDiscoveryFeed
	Discovered    []string
	Scorer        *BlockPeerScorer
	AddedNodes    *AddedNodeStore
	RawBackfill   int
}

// TryPromoteSoloPrimary handshakes an addnode/DNS candidate and runs a headers-first sync round.
// Returns a connected peer on success (caller owns the TCP session).
func TryPromoteSoloPrimary(o SoloAttachOpts) (*headerSyncPeer, error) {
	if o.Journal == nil {
		return nil, nil
	}
	peers := HeaderSyncProbeCandidates(
		DiscoverySnapshot(o.DiscoveryFeed, o.Discovered),
		o.Scorer,
		o.AddedNodes.List(),
	)
	if len(peers) == 0 {
		return nil, nil
	}
	probed, err := probeHeaderSyncPeers(o.Ctx, o.Dialer, peers, o.Params, o.UserAgent, o.LocalServices, 1, o.Scorer, nil)
	if err != nil {
		return nil, err
	}
	if len(probed) == 0 {
		return nil, nil
	}
	peer := probed[0]
	applog.Line("net", fmt.Sprintf("solo: attaching to peer %s (height %d) for header sync", peer.addr, peer.startHeight()))
	if err := DownloadHeaders(
		o.Ctx, peer.mw, o.Params, o.Journal, o.Aux, o.FeeFilters, o.BlockStore, o.RawBackfill, o.RawFill,
		peer.startHeight(), o.DiscoveryFeed, false, o.Scorer, nil,
	); err != nil {
		closeHeaderSyncPeer(peer)
		return nil, err
	}
	return &peer, nil
}

// PeerMgrRelayEnv builds the relay handler env shared by normal and solo multi-peer modes.
func PeerMgrRelayEnv(
	net chain.Network,
	cfg Config,
	j *store.HeaderJournal,
	auxJ *store.HeaderAuxJournal,
	chainPolicy *store.ChainPolicy,
	blockStore *BlockStoreCtx,
	pool *mempool.Pool,
	orphans *mempool.OrphanPool,
	txIx *store.TxIndex,
	rbStore *store.RawBlockStore,
	filterIx *store.BlockFilterIndex,
	peerFeeFilters *FeeFilterSet,
	tipWait *rpc.TipWaiter,
	rawFill *progressiveRawState,
	misbehavior *MisbehaviorTracker,
) RelayEnv {
	return RelayEnv{
		Network: net, FullNode: cfg.FullNode,
		AllowUnverifiedMempool: cfg.AllowUnverifiedMempool, FullRBF: cfg.FullRBF,
		Standard: cfg.Standard, MempoolLimits: cfg.MempoolLimits,
		Journal: j, Aux: auxJ, ChainPolicy: chainPolicy, BlockStore: blockStore,
		Pool: pool, Orphans: orphans, TxIndex: txIx, RawBlocks: rbStore,
		BlockFilters: filterIx,
		PeerFeeFilter: peerFeeFilters, TipWait: tipWait, RawFill: rawFill,
		Misbehavior: misbehavior,
	}
}

// WirePeerMgrBlockCallbacks configures fork probe / chain election on the block store when peerMgr is active.
func WirePeerMgrBlockCallbacks(blockStore *BlockStoreCtx, peerMgr *PeerMgr, j *store.HeaderJournal, p chain.Params) {
	if blockStore == nil || peerMgr == nil || j == nil {
		return
	}
	blockStore.SetForkProbe(func(forkAt int64, _ [32]byte) {
		if peerMgr != nil && j != nil && (blockStore == nil || !ShouldDeferInboundHeaders(blockStore)) {
			peerMgr.RequestForkProbeFromRelays(p, j, forkAt)
		}
	})
	blockStore.SetChainElection(func(ctx context.Context, forkAt int64, forkPrev [32]byte, incoming []wire.DecodedHeader, incomingWork *big.Int) error {
		if peerMgr == nil || j == nil || ShouldDeferInboundHeaders(blockStore) {
			return nil
		}
		return peerMgr.EnsureIncomingForkWins(ctx, j, p, forkAt, forkPrev, incoming, incomingWork)
	})
	blockStore.OnBlockFromPeer = func(addr string, height int64) {
		peerMgr.NotePeerBlockAt(addr, height)
	}
}
