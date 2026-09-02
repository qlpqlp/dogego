// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"strings"
	"time"

	"dogego/chain"
)

// bodyIBDPumpInterval is how often the primary session proactively issues getdata during forward
// body IBD. Core's block download manager runs continuously; DogeGo previously relied only on P2P
// read idle (~4s), which starved when peers kept sending inv/ping/headers chatter.
const bodyIBDPumpInterval = 50 * time.Millisecond

// bodyIBDAssistStallRelaunch rotates block-assist workers when sessions stay up without body progress.
const bodyIBDAssistStallRelaunch = 45 * time.Second

// bodyIBDPumpBatchesPerRound is progressive getdata rounds per proactive pump tick.
const bodyIBDPumpBatchesPerRound = 4

// ensureBodyDownloadArmed clears stale idleFull while headers still lead stored bodies so claimBatch
// and stall recovery cannot deadlock (idleFull disables both fetch and MaybeRecoverIBDStall).
func (s *progressiveRawState) ensureBodyDownloadArmed(bs *BlockStoreCtx) {
	if s == nil || bs == nil || !BodiesBehindHeaders(bs) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.idleFull {
		s.idleFull = false
	}
}

// MaybePumpBodyIBDDownload proactively fetches the next missing bodies on the primary link during
// forward IBD (Core-style continuous download). Returns blocks stored this round.
func MaybePumpBodyIBDDownload(
	ctx context.Context,
	mw *MsgWriter,
	p chain.Params,
	bs *BlockStoreCtx,
	raw *progressiveRawState,
	scorer *BlockPeerScorer,
	book *AddrBook,
	lastPump *time.Time,
) int {
	n, _ := MaybePumpLaneBodyIBDDownload(ctx, mw, p, bs, raw, 0, scorer, book, lastPump)
	return n
}

// MaybePumpLaneBodyIBDDownload runs proactive getdata on any sync lane (primary lane 0 or relay/assist).
// The error is non-nil when the peer stream failed (caller must disconnect; do not issue more getdata).
func MaybePumpLaneBodyIBDDownload(
	ctx context.Context,
	mw *MsgWriter,
	p chain.Params,
	bs *BlockStoreCtx,
	raw *progressiveRawState,
	laneID int,
	scorer *BlockPeerScorer,
	book *AddrBook,
	lastPump *time.Time,
) (int, error) {
	if raw == nil || bs == nil || mw == nil || !raw.bodiesDownloadActive(bs) {
		return 0, nil
	}
	raw.ensureBodyDownloadArmed(bs)
	// Soft-stall hole reclaim: skip the 50ms pump throttle so another lane grabs contiguous+1 ASAP.
	if lastPump != nil && !lastPump.IsZero() && time.Since(*lastPump) < bodyIBDPumpInterval && !raw.HoleReclaimPending() {
		return 0, nil
	}
	if lastPump != nil {
		*lastPump = time.Now()
	}
	rounds := bodyIBDPumpBatchesPerRound
	if raw.throughputBoostActive(bs) {
		rounds = 8
	}
	n, err := raw.tryFetchMissingBatches(ctx, mw, p, bs, laneID, rounds, scorer, book)
	if n > 0 && scorer != nil && mw.PeerAddr != "" {
		scorer.NoteBlocksDelivered(mw.PeerAddr, n)
	}
	if err != nil && !IsBenignShutdownErr(err) && mw.PeerAddr != "" && scorer != nil {
		if shouldRotatePeerForStubBlock(err) {
			penalizeStubBlockPeer(scorer, book, mw.PeerAddr)
		} else if strings.Contains(err.Error(), "bad magic") {
			penalizeWrongNetworkPeer(scorer, book, mw.PeerAddr, err)
		} else if sessionFailureHardFromFetchErr(err) || shouldRotatePeerForForwardIBDFetch(err, blockFetchWantHeight(bs)) {
			penalizeBlockPeer(scorer, book, mw.PeerAddr, true)
		}
	}
	return n, err
}

// blockAssistSessionsStalled reports assist TCP sessions that outlived recent body storage progress.
func blockAssistSessionsStalled(raw *progressiveRawState, reg *AssistPeerRegistry) bool {
	if raw == nil || reg == nil || reg.Count() == 0 {
		return false
	}
	if raw.hasDownloadInFlight() {
		return false
	}
	snap := raw.snapshot()
	lastUnix, _ := snap["last_block_stored_at"].(int64)
	if lastUnix <= 0 {
		return false
	}
	return time.Since(time.Unix(lastUnix, 0)) >= bodyIBDAssistStallRelaunch
}
