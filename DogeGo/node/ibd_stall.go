// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"time"

	"dogego/applog"
)

// ibdStallNoBlockInterval triggers peer refresh when no raw block was stored for this long during IBD.
const ibdStallNoBlockInterval = 15 * time.Minute

const ibdStallNoBlockIntervalEarly = 5 * time.Minute

// ibdStallNoBlockIntervalMid applies during deep body IBD (stored bodies well ahead of headers gap window).
const ibdStallNoBlockIntervalMid = 10 * time.Minute

// ibdStallNoBlockIntervalGenesis applies while height 0 body is missing (pruned-peer rotation must happen quickly).
const ibdStallNoBlockIntervalGenesis = 2 * time.Minute

// ibdStallNoBlockIntervalBodyOnly applies when chainActive caught up but bodies still lag headers
// (post-connect body IBD; stub-pruned peers need faster peer rotation than deep bulk download).
const ibdStallNoBlockIntervalBodyOnly = 90 * time.Second

// ibdStallNoBlockIntervalZeroInflight applies when bodies lag but no getdata claims are outstanding
// (assist/primary stopped issuing batches — recover faster than the mid-IBD 5–10m windows).
const ibdStallNoBlockIntervalZeroInflight = 45 * time.Second

// ibdStallNoBlockIntervalZeroStored applies when this process has stored no blocks yet but bodies lag headers.
const ibdStallNoBlockIntervalZeroStored = 5 * time.Minute

func connectCatchUpComplete(bs *BlockStoreCtx) bool {
	if bs == nil || bs.Utxo == nil {
		return false
	}
	return ConnectCatchUpLag(bs, bs.Utxo) == 0
}

func ibdStallRecoverInterval(bs *BlockStoreCtx, snap map[string]interface{}) time.Duration {
	if bs != nil && NeedsGenesisBlock(bs) {
		return ibdStallNoBlockIntervalGenesis
	}
	if stored, ok := snap["blocks_stored_ibd"].(int64); ok && stored == 0 && bs != nil && BodiesBehindHeaders(bs) {
		if cont := bs.ContiguousRawHeight(); cont >= 1000 {
			return ibdStallNoBlockIntervalZeroStored
		}
	}
	if inflight, ok := snap["in_flight_batches"].(int); ok && inflight == 0 {
		if lastUnix, ok := snap["last_block_stored_at"].(int64); ok && lastUnix > 0 {
			if time.Since(time.Unix(lastUnix, 0)) >= 30*time.Second && bs != nil && BodiesBehindHeaders(bs) {
				return ibdStallNoBlockIntervalZeroInflight
			}
		}
	} else if inflight64, ok := snap["in_flight_batches"].(int64); ok && inflight64 == 0 {
		if lastUnix, ok := snap["last_block_stored_at"].(int64); ok && lastUnix > 0 {
			if time.Since(time.Unix(lastUnix, 0)) >= 30*time.Second && bs != nil && BodiesBehindHeaders(bs) {
				return ibdStallNoBlockIntervalZeroInflight
			}
		}
	}
	if bs != nil && connectCatchUpComplete(bs) && BodiesBehindHeaders(bs) {
		return ibdStallNoBlockIntervalBodyOnly
	}
	if bs == nil || bs.Journal == nil {
		return ibdStallNoBlockInterval
	}
	tip, err := bs.Journal.TipHeight()
	if err != nil || tip < 1 {
		return ibdStallNoBlockInterval
	}
	cont := bs.ContiguousRawHeight()
	if cont < 0 {
		cont = 0
	}
	if bs != nil && ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) && cont < 50_000 {
		return ibdStallNoBlockIntervalEarly
	}
	if tip-cont > forwardIBDParallelWindow && cont < 1000 {
		return ibdStallNoBlockIntervalEarly
	}
	if tip-cont > 10_000 && cont >= 1000 {
		return ibdStallNoBlockIntervalMid
	}
	return ibdStallNoBlockInterval
}

// MaybeRecoverIBDStall requests more peers and refreshes the assist pool when block download appears stuck.
func MaybeRecoverIBDStall(
	mw *MsgWriter,
	pm *PeerMgr,
	raw *progressiveRawState,
	bs *BlockStoreCtx,
	assist *BlockAssistCandidates,
	feed *PeerDiscoveryFeed,
	discovered []string,
	scorer *BlockPeerScorer,
	added []string,
	lastRecover *time.Time,
	launch func() BlockAssistLaunchParams,
	refreshDiscovery func() []string,
	ensureAssist func(),
) {
	if raw == nil || lastRecover == nil || bs == nil || !BodiesBehindHeaders(bs) {
		return
	}
	raw.ensureBodyDownloadArmed(bs)
	snap := raw.snapshot()
	stallWait := ibdStallRecoverInterval(bs, snap)
	if !lastRecover.IsZero() && time.Since(*lastRecover) < stallWait {
		return
	}
	lastUnix, _ := snap["last_block_stored_at"].(int64)
	if lastUnix > 0 && time.Since(time.Unix(lastUnix, 0)) < stallWait {
		return
	}
	// Grace period at IBD start before the first stored block.
	if lastUnix == 0 {
		if started, ok := snap["ibd_elapsed_sec"].(int64); ok && started >= 0 && started < int64(stallWait.Seconds()) {
			return
		}
		if stored, ok := snap["blocks_stored_ibd"].(int64); ok && stored == 0 {
			if elapsed, ok := snap["ibd_elapsed_sec"].(int64); ok && elapsed >= 0 && elapsed < int64(stallWait.Seconds()) {
				return
			}
		}
	}
	var book *AddrBook
	if pm != nil {
		book = addrBookFromPeerMgr(pm)
	}
	if scorer != nil {
		_, _ = raw.maybePenalizeStallingPeer(bs, scorer, book)
	}
	*lastRecover = time.Now()
	applog.Line("block", "IBD stall: no recent block progress; realigning sync cursor and refreshing peers")
	if bs != nil {
		if err := EnsureLocalGenesis(bs); err != nil {
			applog.Line("block", "IBD stall local genesis: "+err.Error())
		}
		ReconcileGenesisWithContiguous(bs)
	}
	if bs != nil {
		if gap := ConnectBodyGapHeight(bs); gap >= 0 {
			if bs.utxoAheadOfStoredBodies() {
				bs.recoverBodiesOnConnectGap(gap)
			} else {
				bs.recoverBodiesOnConnectGapFull(gap)
			}
		}
		prev := bs.ContiguousRawHeight()
		var refreshed int64
		if bs.utxoAheadOfStoredBodies() {
			refreshed = bs.RampReplayContiguousFromDisk()
		} else {
			refreshed = bs.RefreshContiguousTip()
		}
		if refreshed > prev {
			applog.Line("block", fmt.Sprintf("IBD stall recovery: contiguous tip %d → %d", prev, refreshed))
		}
	}
	raw.realignProbeToLowestMissing(bs)
	if refreshDiscovery != nil {
		if fresh := refreshDiscovery(); len(fresh) > 0 {
			discovered = fresh
		}
	}
	RequestGetAddrFromPeers(mw, pm)
	if assist != nil {
		before := assist.Len()
		RefreshBlockAssistPool(assist, DiscoverySnapshot(feed, discovered), pm, scorer, bs, added)
		after := assist.Len()
		if after != before {
			applog.Line("block", fmt.Sprintf("IBD stall recovery: assist pool %d → %d", before, after))
		}
	}
	if ensureAssist != nil {
		ensureAssist()
	}
	if launch != nil {
		p := launch()
		if p.Candidates != nil && p.Candidates.Len() > 0 {
			MaybeEnsureBlockAssistWorkers(p)
		}
	}
}
