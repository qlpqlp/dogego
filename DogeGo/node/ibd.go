// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/store"
	"dogego/wire"
)

// IBD tuning (Core downloads blocks in parallel from several peers; historical catch-up is sequential in height).
const (
	progressiveBatchSize    = 16 // per-peer in-flight cap (Core MAX_BLOCKS_IN_TRANSIT_PER_PEER)
	progressiveBatchSizeMax = 32 // scaled cap when many parallel sync lanes share load
	// ibdGetDataBatch is ltcd-style (netsync/manager.go fetchHeaderBlocks): one getdata
	// keeps the peer busy. ltcd allows up to wire.MaxInvPerMsg (50_000); 256 is large
	// enough to fill a fast pipe without a 50k inv that some Dogecoin peers drop.
	ibdGetDataBatch = 256
	// minInFlightBlocks matches ltcd netsync: send the next getdata when requested
	// blocks on that peer drop below this, so the TCP pipe does not drain between batches.
	minInFlightBlocks          = 10
	tipBackfillDeferGap        = 512
	blockDownloadWindow        = 1024 // Core validation.h BLOCK_DOWNLOAD_WINDOW
	forwardIBDParallelWindow   = 4096 // wider stripe span when many parallel lanes share load
	invBlockFetchFrontierSlack = 128  // defer inv getdata for blocks ahead of contiguous bodies during forward IBD
	headerCatchUpPeerLead      = 32   // peer within this many headers of local tip → defer header sync while bodies lag
	headersSyncRoundsBodiesLag = 2    // max getheaders rounds when peer ≤ tip but bodies still behind
	minBlockAssistWorkers      = 3
	maxBlockAssistWorkers      = 24
)

// EffectiveProgressiveBatchSize scales getdata batch size with parallel sync lanes (Core: more in-flight per peer when several links download).
func EffectiveProgressiveBatchSize(syncLanes int) int {
	n := progressiveBatchSize
	if syncLanes > 2 {
		n += (syncLanes - 2) * 8
	}
	if n > progressiveBatchSizeMax {
		return progressiveBatchSizeMax
	}
	if n < 16 {
		return 16
	}
	return n
}

// ShouldDeferTxIndexOnPut skips per-txid IndexBlock during deep body IBD (index on ConnectBlock instead).
// Core's LevelDB txindex is cheap on the download path; DogeGo's one-file-per-txid layout is not.
func ShouldDeferTxIndexOnPut(bs *BlockStoreCtx) bool {
	if bs == nil || bs.Journal == nil || !BodiesBehindHeaders(bs) {
		return false
	}
	tip, err := bs.Journal.TipHeight()
	if err != nil || tip < 0 {
		return false
	}
	cont := bs.ContiguousRawHeight()
	gap := tip - cont
	if cont < 0 {
		gap = tip + 1
	}
	// Once bodies lag headers by more than the tip-backfill defer gap, Put must stay download-only.
	return gap > tipBackfillDeferGap
}

// ShouldPauseHeaderCatchUpForBodyIBD reports Core-style pausing of getheaders while deep forward block IBD runs.
// Headers keep advancing until the assumevalid height is on the local tip (mainnet default 5,050,000) so
// script-skip can unlock; only then may getheaders yield to body download when bodies lag far behind.
func ShouldPauseHeaderCatchUpForBodyIBD(bs *BlockStoreCtx, peerStart int32) bool {
	if bs == nil || bs.Journal == nil || !BodiesBehindHeaders(bs) {
		return false
	}
	tip, err := bs.Journal.TipHeight()
	if err != nil || tip < 0 {
		return false
	}
	cont := bs.ContiguousRawHeight()
	gap := tip - cont
	if cont < 0 {
		gap = tip + 1
	}
	minTip := headerBodyIBDPauseMinTip(bs)
	if gap > 50_000 && tip >= minTip {
		return true
	}
	return ShouldDeferHeaderSyncWhileBodiesLag(tip, peerStart, true)
}

// headerBodyIBDPauseMinTip is the header tip height required before deep body IBD may pause getheaders.
// With assumevalid configured, keep headers moving until that height (or resolution) so ConnectBlock can
// skip scripts on buried history - matching Core IBD where AV unlocks after headers include the hash.
func headerBodyIBDPauseMinTip(bs *BlockStoreCtx) int64 {
	const legacyMinTip int64 = 500_000
	if bs == nil || bs.AssumeValid == nil {
		return legacyMinTip
	}
	av := bs.AssumeValid
	hex := strings.TrimSpace(av.HashHex())
	if hex == "" {
		// verify-all / disabled assumevalid: legacy pause once headers are far ahead
		return legacyMinTip
	}
	if h := av.Height(); h >= 0 {
		return h
	}
	if defH := consensus.AssumeValidHeightForHash(hex); defH > 0 {
		return defH
	}
	// Custom unresolved hash: do not pause on the old 500k rule; keep pulling headers until resolved.
	return int64(^uint64(0) >> 1) // MaxInt64
}

// ShouldRunHeaderAdvanceWatchdog is false while body IBD owns the sync pipeline (headers already far ahead).
func ShouldRunHeaderAdvanceWatchdog(j *store.HeaderJournal, bs *BlockStoreCtx, peerStart int32) bool {
	if ShouldPauseHeaderCatchUpForBodyIBD(bs, peerStart) {
		return false
	}
	return shouldContinueHeaderCatchUpDuringIBD(j, peerStart)
}

// blockFetchInvTypes returns getdata inv types for full block download (Dogecoin: no segwit witness round).
func blockFetchInvTypes(p chain.Params) []uint32 {
	_ = p
	return []uint32{wire.InvTypeBlock}
}

// IdleFetchBatchesPerRound is how many progressive getdata rounds assist/relay peers run per idle timeout (primary uses 3).
func IdleFetchBatchesPerRound(bs *BlockStoreCtx) int {
	if bs != nil && ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		return 4
	}
	return 2
}

// shouldRefillGetData is ltcd's minInFlightBlocks check: request more before the
// current getdata fully drains so the peer's send buffer stays busy.
func shouldRefillGetData(pending int) bool {
	return pending < minInFlightBlocks
}

// shouldPipelineGetData enables mid-batch getdata refill during download-first IBD.
func shouldPipelineGetData(bs *BlockStoreCtx) bool {
	return ShouldDeferConnectForBodyDownload(bs) || ShouldPauseHeaderCatchUpForBodyIBD(bs, 0)
}

// EffectiveProgressiveBatchSizeForIBD sizes getdata during body IBD.
// Download-first (headers far ahead): fat getdata like ltcd fetchHeaderBlocks.
// tryFetchMissingBatches then refills when pending drops below minInFlightBlocks.
func EffectiveProgressiveBatchSizeForIBD(bs *BlockStoreCtx, syncLanes int) int {
	n := EffectiveProgressiveBatchSize(syncLanes)
	if bs == nil {
		return n
	}
	cont := bs.ContiguousRawHeight()
	if ShouldDeferConnectForBodyDownload(bs) || ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		if cont < 32 {
			if n > 8 {
				return 8
			}
			return n
		}
		return ibdGetDataBatch
	}
	if cont >= 1000 {
		return n
	}
	if n > 8 {
		n = 8
	}
	if cont < 32 && n > 4 {
		n = 4
	}
	return n
}

// EffectiveBlockSyncWorkers returns parallel block-download TCP sessions (block-assist pool).
// configured 0 derives from maxOutbound (Core-style: several outbound links for blocks).
func EffectiveBlockSyncWorkers(maxOutbound, configured int) int {
	return EffectiveBlockSyncWorkersOpt(maxOutbound, configured, false)
}

// EffectiveBlockSyncWorkersOpt is EffectiveBlockSyncWorkers with optional IBD optimize boost
// (more assist peers when auto-sized, like Core prioritizing getdata during IBD).
func EffectiveBlockSyncWorkersOpt(maxOutbound, configured int, ibdOptimize bool) int {
	if configured > 0 {
		if configured > maxBlockAssistWorkers {
			return maxBlockAssistWorkers
		}
		if configured < 1 {
			return minBlockAssistWorkers
		}
		return configured
	}
	n := minBlockAssistWorkers
	if maxOutbound > 1 {
		n = maxOutbound - 1
		if n > maxBlockAssistWorkers {
			n = maxBlockAssistWorkers
		}
		if n < minBlockAssistWorkers {
			n = minBlockAssistWorkers
		}
	}
	if ibdOptimize {
		// Prefer saturating outbound capacity for bodies during IBD (headers-first + parallel getdata).
		boost := maxOutbound
		if boost < minBlockAssistWorkers+2 {
			boost = minBlockAssistWorkers + 2
		}
		if boost > n {
			n = boost
		}
		if n > maxBlockAssistWorkers {
			n = maxBlockAssistWorkers
		}
	}
	return n
}

// headerRewindDeferBodyLagMin is how far the header tip may lead contiguous bodies before automatic
// header truncates are skipped (forward block IBD should not chop a 500k header tip while bodies are at 8k).
const headerRewindDeferBodyLagMin int64 = 4096

// shouldDeferHeaderTipTruncateDuringBodyIBD skips header journal truncates that would not help
// forward block download (header tip far ahead of stored bodies).
func shouldDeferHeaderTipTruncateDuringBodyIBD(bs *BlockStoreCtx, tip, rewindTo int64) bool {
	if bs == nil || !BodiesBehindHeaders(bs) {
		return false
	}
	cont := bs.ContiguousRawHeight()
	if cont < 0 || tip <= cont+headerRewindDeferBodyLagMin {
		return false
	}
	_ = rewindTo
	return true
}

// BodiesBehindHeaders reports whether raw block files do not yet cover the local header chain.
func BodiesBehindHeaders(bs *BlockStoreCtx) bool {
	if bs == nil || bs.Journal == nil || bs.Raw == nil {
		return false
	}
	tip, err := bs.Journal.TipHeight()
	if err != nil || tip < 0 {
		return false
	}
	net := bs.chainNet()
	if !store.HasStoredBodyAtHeight(bs.Journal, bs.Raw, 0, net) {
		return true
	}
	if tip < 1 {
		return false
	}
	return bs.ContiguousRawHeight() < tip
}

// ShouldDeferConnectForBodyDownload is Core-style download-first IBD: fetch headers and block
// bodies before ConnectBlock / txindex. Connect and indexes run once bodies are within
// BLOCK_DOWNLOAD_WINDOW of the header tip (or after IBD). UTXO-ahead snapshot replay still
// fetches missing bodies and is not treated as connect work.
func ShouldDeferConnectForBodyDownload(bs *BlockStoreCtx) bool {
	if bs == nil || !BodiesBehindHeaders(bs) {
		return false
	}
	if bs.utxoAheadOfStoredBodies() {
		return false
	}
	return bs.forwardIBDGap() > blockDownloadWindow
}

// NeedsGenesisBlock is true when height 0 is missing from rawblocks/ but headers exist.
func NeedsGenesisBlock(bs *BlockStoreCtx) bool {
	if bs == nil || bs.Journal == nil || bs.Raw == nil {
		return false
	}
	return !store.HasStoredBodyAtHeight(bs.Journal, bs.Raw, 0, bs.chainNet())
}

// HasLocalHeaderChain reports whether headers.bin has at least genesis stored.
func HasLocalHeaderChain(j *store.HeaderJournal) bool {
	if j == nil {
		return false
	}
	tip, err := j.TipHeight()
	return err == nil && tip >= 0
}

// ShouldDeferHeaderSyncWhileBodiesLag is true when Core would prioritize block download over
// pulling more headers (local header chain already matches the peer's advertised height).
func ShouldDeferHeaderSyncWhileBodiesLag(localTip int64, peerStart int32, bodiesBehind bool) bool {
	if !bodiesBehind || localTip < 1 || peerStart <= 0 {
		return false
	}
	return int64(peerStart) <= localTip+headerCatchUpPeerLead
}

// headersSyncMaxRounds limits header-only rounds when local headers already cover the peer's
// advertised height but block bodies still need forward IBD (prioritize getdata over more headers).
func headersSyncMaxRounds(localTip int64, peerStart int32, bodiesBehind bool) int {
	if localTip < 1 || !bodiesBehind {
		return 4096
	}
	if peerStart > 0 && int64(peerStart) <= localTip {
		return headersSyncRoundsBodiesLag
	}
	return 4096
}

// ConnectNextHeight is the next block height the UTXO set expects (chainActive+1).
func ConnectNextHeight(bs *BlockStoreCtx) int64 {
	if bs == nil || bs.Utxo == nil {
		return -1
	}
	return bs.Utxo.TipHeight() + 1
}

// ConnectFrontierHeight is the height ConnectTip should work on now. When a UTXO snapshot is
// ahead of stored bodies, this is contiguous+1 (body replay) instead of utxo.Tip+1.
func ConnectFrontierHeight(bs *BlockStoreCtx) int64 {
	if bs == nil || bs.Utxo == nil {
		return -1
	}
	next := bs.Utxo.TipHeight() + 1
	if bs.Journal != nil && bs.Raw != nil {
		if cont := bs.ContiguousRawHeight(); cont >= 0 && bs.Utxo.TipHeight() > cont {
			next = cont + 1
		}
	}
	return next
}

// ConnectBodyGapHeight returns the first height chainActive cannot connect past because the raw
// body is missing, or -1 when connect is not blocked on a missing body.
func ConnectBodyGapHeight(bs *BlockStoreCtx) int64 {
	if bs == nil || bs.Journal == nil || bs.Raw == nil {
		return -1
	}
	next := ConnectFrontierHeight(bs)
	if next < 0 {
		return -1
	}
	tip, err := bs.Journal.TipHeight()
	if err != nil || next > tip {
		return -1
	}
	if store.HasStoredBodyAtHeight(bs.Journal, bs.Raw, next, bs.chainNet()) {
		return -1
	}
	return next
}

// PreferConnectFrontierMissing prioritizes the next body needed for ConnectTip over forward orphan gaps.
func PreferConnectFrontierMissing(j *store.HeaderJournal, rs *store.RawBlockStore, low, connectNext int64, net chain.Network) int64 {
	if connectNext < 0 || j == nil || rs == nil {
		return low
	}
	if store.HasStoredBodyAtHeight(j, rs, connectNext, net) {
		return low
	}
	if low < 0 || connectNext < low {
		return connectNext
	}
	return low
}

// LowestMissingForIBD returns the height forward block download should target next.
func LowestMissingForIBD(j *store.HeaderJournal, rs *store.RawBlockStore, contiguous, tip int64, bs *BlockStoreCtx) (int64, error) {
	if j == nil || rs == nil {
		return -1, nil
	}
	tip = capBodyDownloadTip(bs, tip)
	net := chain.MainnetDogecoin
	if bs != nil {
		net = bs.chainNet()
	}
	low, err := store.LowestMissingAfterContiguous(j, rs, contiguous, tip, net)
	if low < 0 {
		searchStart := store.LowestMissingSearchStart(j, rs, contiguous, net)
		low, err = store.LowestMissingBlockHeightFrom(j, rs, searchStart, tip, net)
	}
	if err != nil {
		return low, err
	}
	connectNext := int64(-1)
	if bs != nil {
		connectNext = ConnectFrontierHeight(bs)
	}
	return PreferConnectFrontierMissing(j, rs, low, connectNext, net), nil
}

// ShouldDeferTipBackfill skips startup tip-aligned getdata when a large historical gap remains
// (Core IBD fills forward from the last common block; tip-first batches create orphan stores).
func ShouldDeferTipBackfill(headerTip, contiguousRaw int64) bool {
	if headerTip < 1 {
		return false
	}
	if contiguousRaw < 0 {
		contiguousRaw = 0
	}
	gap := headerTip - contiguousRaw
	return gap > tipBackfillDeferGap
}

// shouldFillContiguousFrontierFirst reports whether all download lanes should focus on the same
// height window starting at lowMissing (Core forward IBD) instead of striping across a wide range.
func shouldFillContiguousFrontierFirst(bs *BlockStoreCtx, lowMissing int64) bool {
	if bs == nil || bs.Journal == nil || lowMissing < 0 {
		return false
	}
	if gap := ConnectBodyGapHeight(bs); gap >= 0 {
		return true
	}
	if lowMissing == 0 {
		return true // genesis and early heights: all lanes fill from 0, not stripes at 1024+
	}
	cont := bs.ContiguousRawHeight()
	tip, err := bs.Journal.TipHeight()
	if err != nil || tip < 0 {
		return false
	}
	if lowMissing > cont+1 {
		return true
	}
	if tip < 1 {
		return lowMissing > 0
	}
	return ShouldDeferTipBackfill(tip, cont)
}

// ShouldSuppressInvTxFetchDuringIBD skips inv/tx getdata while stored bodies lag the header tip.
// Core does not fill the mempool during IBD. The old tip<500k quiet window stopped applying once
// headers reached assumevalid (~5.05M) while bodies were still near genesis.
func ShouldSuppressInvTxFetchDuringIBD(bs *BlockStoreCtx) bool {
	return bs != nil && BodiesBehindHeaders(bs)
}

// ShouldDeferInvBlockFetch skips inv-driven block getdata far ahead of the contiguous raw
// frontier during forward IBD (Core prioritizes sequential fill from the lowest missing height).
func ShouldDeferInvBlockFetch(bs *BlockStoreCtx, hashLE [32]byte) bool {
	if bs == nil || bs.Journal == nil || bs.Raw == nil {
		return false
	}
	tip, err := bs.Journal.TipHeight()
	if err != nil || tip < 1 {
		return false
	}
	low, err := LowestMissingForIBD(bs.Journal, bs.Raw, bs.ContiguousRawHeight(), tip, bs)
	if err != nil || low < 0 {
		return false
	}
	if !shouldFillContiguousFrontierFirst(bs, low) {
		return false
	}
	h, err := bs.Journal.HeightByBlockHashLE(hashLE)
	if err != nil {
		return true
	}
	if gap := ConnectBodyGapHeight(bs); gap >= 0 && h > gap+invBlockFetchFrontierSlack {
		return true
	}
	cont := bs.ContiguousRawHeight()
	if cont >= 0 && h > cont+invBlockFetchFrontierSlack {
		return true
	}
	maxFetch := cont + forwardIBDParallelWindow
	if maxFetch < low+forwardIBDParallelWindow-1 {
		maxFetch = low + forwardIBDParallelWindow - 1
	}
	return h > maxFetch
}

// capBodyDownloadTip limits forward getdata to the UTXO snapshot tip while bodies lag during
// snapshot replay (better-than-Core: finish replay before chasing the full header chain).
func capBodyDownloadTip(bs *BlockStoreCtx, headerTip int64) int64 {
	if bs == nil || headerTip < 0 || !bs.utxoAheadOfStoredBodies() {
		return headerTip
	}
	if bs.Utxo == nil {
		return headerTip
	}
	utxoTip := bs.Utxo.TipHeight()
	if utxoTip < 0 || utxoTip >= headerTip {
		return headerTip
	}
	return utxoTip
}

// invBlockFetchWorthwhile reports whether an inv message has any block hash worth fetching now.
func invBlockFetchWorthwhile(bs *BlockStoreCtx, entries []wire.InvEntry) bool {
	for _, e := range entries {
		switch e.Type {
		case wire.InvTypeBlock, wire.InvTypeWitnessBlock:
			height := int64(-1)
			if bs.Journal != nil {
				if h, err := bs.Journal.HeightByBlockHashLE(e.Hash); err == nil {
					height = h
				}
			}
			if bs.rawBodyPresent(e.Hash, height) {
				continue
			}
			if !ShouldDeferInvBlockFetch(bs, e.Hash) {
				return true
			}
		}
	}
	return false
}

// forwardIBDStripeTip caps parallel download stripes during forward IBD when headers are far
// ahead of contiguous bodies. Without this, lane 1+ would fetch height ~tip/3 while height 2 is missing.
func forwardIBDStripeTip(bs *BlockStoreCtx, lowMissing, tip int64) int64 {
	if bs == nil || tip < 0 || lowMissing < 0 {
		return tip
	}
	if !ShouldDeferTipBackfill(tip, bs.ContiguousRawHeight()) {
		return tip
	}
	win := int64(forwardIBDParallelWindow)
	if shouldFillContiguousFrontierFirst(bs, lowMissing) {
		// One fat getdata (ltcd-style) past the hole — not the full 1024-high window,
		// which let other lanes fetch ~1000 heights ahead while height N was still missing.
		win = int64(ibdGetDataBatch)
		if win < 16 {
			win = 16
		}
	}
	hi := lowMissing + win - 1
	if hi > tip {
		hi = tip
	}
	return hi
}

// syncStripeBounds assigns each parallel downloader a contiguous height sub-range of [lowMissing, tip]
// so batched getdata stays sequential within a stripe (Core-style parallel block fetch).
func syncStripeBounds(lowMissing, tip int64, workerID, workerCount int) (lo, hi int64, ok bool) {
	if tip < 0 || lowMissing < 0 || lowMissing > tip {
		return 0, 0, false
	}
	if workerCount <= 1 {
		return lowMissing, tip, true
	}
	if workerID < 0 || workerID >= workerCount {
		return 0, 0, false
	}
	n := tip - lowMissing + 1
	span := n / int64(workerCount)
	if span < 1 {
		span = 1
	}
	lo = lowMissing + int64(workerID)*span
	hi = lo + span - 1
	if workerID == workerCount-1 {
		hi = tip
	}
	if lo > tip {
		return 0, 0, false
	}
	return lo, hi, true
}

// rangeHasMissingBlock reports whether any height in [lo, hi] lacks a stored raw block.
func rangeHasMissingBlock(j *store.HeaderJournal, raw *store.RawBlockStore, lo, hi int64, net chain.Network) (bool, error) {
	if j == nil || raw == nil || lo > hi {
		return false, nil
	}
	for h := lo; h <= hi; h++ {
		if !store.HasStoredBodyAtHeight(j, raw, h, net) {
			return true, nil
		}
	}
	return false, nil
}
