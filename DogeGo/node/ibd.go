// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strings"
	"time"

	"dogego/chain"
	"dogego/consensus"
	"dogego/store"
	"dogego/wire"
)

// IBD body download: adaptive multi-peer scheduler (Core parallelism + earned fat getdata).
// Bodies stage in RAM write-behind, then flush to disk off the P2P read path.
//
// Core MAX_BLOCKS_IN_TRANSIT_PER_PEER=16 is the safe start. Early Dogecoin bodies are
// hundreds of bytes, so proven-fast peers may rise toward ibdPeerInFlightMax (256).
// Slow / stalling peers drop to 4–8 so they cannot bury contiguous+1 (head-of-line).
//
// Global in-flight is capped at maxIBDFetchWindow (4096) across all lanes. The contiguous
// hole always uses a Core-sized 16-inv getdata; ahead lanes use their per-peer budget.
const (
	progressiveBatchSize    = 16 // near-tip / inv (Core MAX_BLOCKS_IN_TRANSIT_PER_PEER)
	progressiveBatchSizeMax = 32 // scaled cap near tip when several lanes share inv load
	// ibdGetDataBatch is the hard per-peer ceiling during body IBD (adaptive max).
	ibdGetDataBatch = 256
	// Adaptive per-peer in-flight budgets (blocks outstanding to one peer).
	ibdPeerInFlightInitial   = 16
	ibdPeerInFlightSlowFloor = 4
	ibdPeerInFlightSlow      = 8
	ibdPeerInFlightFast      = 64
	ibdPeerInFlightMax       = 256 // == ibdGetDataBatch
	// minInFlightBlocks matches ltcd netsync: send the next getdata when requested
	// blocks on that peer drop below this, so the TCP pipe does not drain between batches.
	minInFlightBlocks   = 10
	tipBackfillDeferGap = 512
	blockDownloadWindow = 1024 // Core validation.h BLOCK_DOWNLOAD_WINDOW (floor)
	// maxIBDFetchWindow is the global in-flight height cap across all lanes.
	maxIBDFetchWindow = 4096
	// forwardIBDParallelWindow is the boundary where the forward (header-led)
	// body gap is considered "deep IBD" for connect deferral decisions.
	// Keep it aligned with Core's smaller download window so unit tests and
	// operator expectations don't treat a ~5k header orphan window as "near".
	forwardIBDParallelWindow   = blockDownloadWindow
	invBlockFetchFrontierSlack = 128 // defer inv getdata for blocks ahead of contiguous bodies during forward IBD
	headerCatchUpPeerLead      = 32  // peer within this many headers of local tip â†’ defer header sync while bodies lag
	headersSyncRoundsBodiesLag = 2   // max getheaders rounds when peer â‰¤ tip but bodies still behind
	minBlockAssistWorkers      = 3
	maxBlockAssistWorkers      = 24
	// Delivery EWMA window for raising/lowering per-peer in-flight budgets.
	ibdPeerDeliveryWindow = 20 * time.Second
	// softStallEscalateCount hard-rotates a peer after this many soft-stalls on the same hole
	// without contiguous advance (busy-lane soft-forever was collapsing throughput).
	softStallEscalateCount = 5
	// maxFrontierClaimPeers is how many lanes may getdata the contiguous hole at once
	// (Core FindNextBlocksToDownload lets several peers race the tip window).
	maxFrontierClaimPeers = 4
	// ibdPeerByteCap limits estimated outstanding body bytes per peer (Core ~16 mid-size blocks).
	ibdPeerByteCap = 8 << 20
	// ibdManyPeersBudgetCap prefers more peers at moderate in-flight once the assist pool is fat.
	ibdManyPeersBudgetCap   = 128
	ibdManyPeersWorkerFloor = 12
	// ibdWriteBehindClaimPauseFrac pauses new getdata when RAM write-behind is this full.
	// 0 disables the pause: stage() already blocks when the buffer is full, and pausing
	// claims starved the TCP pipe (~Core keeps requesting while flush catches up).
	ibdWriteBehindClaimPauseFrac = 0
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

// ShouldRunDedicatedHeaderDespiteBodyPause keeps one headers-only peer alive when local
// header tip still trails the network, even while body peers stay getdata-focused.
// Without this, pause latched after assumevalid and left tip slightly behind peers forever.
func ShouldRunDedicatedHeaderDespiteBodyPause(bs *BlockStoreCtx, peerStart int32) bool {
	if bs == nil || bs.Journal == nil || peerStart <= 0 {
		return false
	}
	if !ShouldPauseHeaderCatchUpForBodyIBD(bs, peerStart) {
		return false
	}
	tip, err := bs.Journal.TipHeight()
	if err != nil || tip < 0 {
		return true
	}
	// More than one headers message (~2000) behind the best peer start height.
	return int64(peerStart) > tip+2000
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
		return 8
	}
	return 2
}

// shouldRefillGetData is ltcd's minInFlightBlocks check: request more before the
// current getdata fully drains so the peer's send buffer stays busy.
func shouldRefillGetData(pending int) bool {
	return pending < minInFlightBlocks
}

// getdataRefillThreshold is ltcd minInFlightBlocks (10) for Core-sized 16-block getdata.
// Fat IBD batches refill at 1/4 remaining so the peer send buffer never drains (ltcd
// refills at 10 of ~500). Half-full was too late once 24 lanes shared one window.
// For Core-sized 16–32 budgets, refill at 3/4 so lanes do not sit half-empty between
// claim rounds under multi-peer window contention.
func getdataRefillThreshold(batchCap int) int {
	if batchCap >= 64 {
		th := batchCap / 4
		if th < minInFlightBlocks {
			return minInFlightBlocks
		}
		return th
	}
	if batchCap > minInFlightBlocks {
		th := (batchCap * 3) / 4
		if th < minInFlightBlocks {
			return minInFlightBlocks
		}
		return th
	}
	return minInFlightBlocks
}

func shouldRefillGetDataAt(pending, batchCap int) bool {
	return pending < getdataRefillThreshold(batchCap)
}

// ibdBodyFetchWindow is how far ahead of the contiguous hole getdata may run.
// Cap at maxIBDFetchWindow (4096). Prefer workers × initial budget, never inflated lane IDs.
func ibdBodyFetchWindow(bs *BlockStoreCtx, workers int) int64 {
	if workers < 1 {
		workers = maxBlockAssistWorkers
	}
	if workers > maxBlockAssistWorkers+1 {
		workers = maxBlockAssistWorkers + 1
	}
	batch := EffectiveProgressiveBatchSizeForIBD(bs, workers)
	if batch < 1 {
		batch = ibdPeerInFlightInitial
	}
	win := int64(workers) * int64(batch)
	// Deep IBD: allow the full global window so 16–24 peers × adaptive budgets stay busy.
	if bs != nil && (ShouldDeferConnectForBodyDownload(bs) || ShouldPauseHeaderCatchUpForBodyIBD(bs, 0)) {
		win = int64(maxIBDFetchWindow)
	}
	if win < int64(blockDownloadWindow) {
		win = int64(blockDownloadWindow)
	}
	if win > int64(maxIBDFetchWindow) {
		win = int64(maxIBDFetchWindow)
	}
	return win
}

// shouldSkipDiskBodyProbe is true during download-first IBD: treat heights after
// contiguous coverage as missing unless already in-flight. Per-file Stat/locator
// probes on every claim froze getdata at Core-sized ~16 leftover hashes per peer.
func shouldSkipDiskBodyProbe(bs *BlockStoreCtx) bool {
	return bs != nil && (ShouldDeferConnectForBodyDownload(bs) || ShouldPauseHeaderCatchUpForBodyIBD(bs, 0))
}

// shouldPipelineGetData enables mid-batch getdata refill during download-first IBD.
func shouldPipelineGetData(bs *BlockStoreCtx) bool {
	return ShouldDeferConnectForBodyDownload(bs) || ShouldPauseHeaderCatchUpForBodyIBD(bs, 0)
}

// EffectiveProgressiveBatchSizeForIBD sizes the default getdata claim during body IBD.
// Deep IBD starts at Core's 16; per-peer adaptive budgets (via progressiveRawState) may
// raise individual lanes toward ibdPeerInFlightMax. tryFetchMissingBatches refills when
// pending drops below getdataRefillThreshold(peerBudget).
func EffectiveProgressiveBatchSizeForIBD(bs *BlockStoreCtx, syncLanes int) int {
	n := EffectiveProgressiveBatchSize(syncLanes)
	if bs == nil {
		return n
	}
	cont := bs.ContiguousRawHeight()
	if ShouldDeferConnectForBodyDownload(bs) || ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		if cont < 32 {
			if n > ibdPeerInFlightSlow {
				return ibdPeerInFlightSlow
			}
			return n
		}
		return ibdPeerInFlightInitial
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
		// Saturate body-download lanes: each assist pulls an adaptive 16–256 getdata.
		// Leave one outbound slot for headers/relay when the budget is small; with a
		// Core-or-larger outbound cap, run the full assist pool (up to 24).
		target := maxOutbound - 1
		if target < minBlockAssistWorkers+2 {
			target = minBlockAssistWorkers + 2
		}
		if maxOutbound >= 16 {
			target = maxBlockAssistWorkers
		}
		if target > n {
			n = target
		}
		if n > maxBlockAssistWorkers {
			n = maxBlockAssistWorkers
		}
		if maxOutbound > 1 && n > maxOutbound-1 {
			n = maxOutbound - 1
			if n < minBlockAssistWorkers {
				n = minBlockAssistWorkers
			}
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
	if low < 0 {
		return connectNext
	}
	if connectNext < low {
		// Download-first IBD: UTXO at 0 with bodies at 50k+ must not collapse the
		// getdata window back to height 1 (claims then sit inside already-stored
		// coverage and fetch nothing).
		if low-connectNext > int64(blockDownloadWindow) {
			return low
		}
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
	if shouldSkipDiskBodyProbe(bs) && contiguous >= 0 {
		low := contiguous + 1
		if low > tip {
			return -1, nil
		}
		connectNext := int64(-1)
		if bs != nil {
			connectNext = ConnectFrontierHeight(bs)
		}
		return PreferConnectFrontierMissing(j, rs, low, connectNext, net), nil
	}
	low, err := store.LowestMissingAfterContiguous(j, rs, contiguous, tip, net)
	if err != nil {
		return low, err
	}
	if low < 0 && bs != nil && contiguous >= 0 && contiguous < tip {
		bs.extendContiguousIfNextStored()
		contiguous = bs.ContiguousRawHeight()
		low, err = store.LowestMissingAfterContiguous(j, rs, contiguous, tip, net)
		if err != nil {
			return low, err
		}
	}
	if low < 0 && contiguous < tip {
		if contiguous < 0 {
			low = 0
		} else {
			low = contiguous + 1
		}
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
	maxFetch := cont + int64(blockDownloadWindow)
	if maxFetch < low+int64(blockDownloadWindow)-1 {
		maxFetch = low + int64(blockDownloadWindow) - 1
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
	return forwardIBDStripeTipFor(bs, lowMissing, tip, 0)
}

func forwardIBDStripeTipFor(bs *BlockStoreCtx, lowMissing, tip int64, workers int) int64 {
	if bs == nil || tip < 0 || lowMissing < 0 {
		return tip
	}
	if !ShouldDeferTipBackfill(tip, bs.ContiguousRawHeight()) {
		return tip
	}
	win := int64(forwardIBDParallelWindow)
	if shouldFillContiguousFrontierFirst(bs, lowMissing) {
		// Core FindNextBlocksToDownload: every peer walks the same 1024-block window
		// from the hole. Stall-cancel still frees the contiguous height.
		win = ibdBodyFetchWindow(bs, workers)
	}
	hi := lowMissing + win - 1
	if hi > tip {
		hi = tip
	}
	return hi
}

// chunkLaneForWorker maps a sync/relay lane ID onto the parallel chunk worker space
// [0, workers). Lane 0 stays 0 (frontier preference). Relay IDs (>=workers) map onto
// ahead-only lanes 1..workers-1 so they keep pulling fat getdata toward tip.
func chunkLaneForWorker(workerID, workers int) int {
	if workers < 1 {
		return 0
	}
	if workerID < 0 {
		return 0
	}
	if workerID < workers {
		return workerID
	}
	if workers == 1 {
		return 0
	}
	return 1 + (workerID % (workers - 1))
}

// lane0AliveLocked reports whether the primary body lane currently has a peer address.
func (s *progressiveRawState) lane0AliveLocked() bool {
	if s == nil || s.laneAddr == nil {
		return false
	}
	return s.laneAddr[0] != ""
}

// holeFillBatchSize is how many heights the hole-owning lane may claim from contiguous+1.
// Soft-open holes and deep body IBD use a fatter batch so the contiguous tip advances
// at Core-or-better speed (peer budgets already go to 64–256; a hard 16-wide hole starved IBD).
func (s *progressiveRawState) holeFillBatchSize(lowMissing int64, batchCap int) int {
	if batchCap < 1 {
		batchCap = progressiveBatchSize
	}
	softOpen := false
	if s != nil {
		s.mu.Lock()
		softOpen = s.softStallFrontier >= 0 && s.softStallFrontier == lowMissing
		s.mu.Unlock()
	}
	target := progressiveBatchSize // 16
	if softOpen {
		target = 64
	} else if batchCap > progressiveBatchSize {
		// Deep IBD: let the hole lane use more of the peer budget (Core window is 16;
		// we already run larger ahead stripes — the tip hole must not stay 16-wide).
		target = 64
		if batchCap < target {
			target = batchCap
		}
	}
	if target > batchCap {
		target = batchCap
	}
	if target < progressiveBatchSize {
		target = progressiveBatchSize
		if target > batchCap {
			target = batchCap
		}
	}
	return target
}

// mayClaimContiguousHole reports whether this lane may ask for contiguous+1.
// Lane 0 owns the hole when alive and actively downloading; after soft-stall, a dead
// primary, or an idle lane-0 owner with a free hole, any lane may reclaim it so gap-fill
// cannot stall while assists only scrape ahead remnants.
//
// Critical: the peer that just soft-stalled the hole must not re-claim it. Live mainnet
// hung when lane 0 soft-released contiguous+1 then immediately re-grabbed it forever
// while assists only filled ahead remnants.
//
// During deep IBD, up to maxFrontierClaimPeers lanes may race the same hole height
// (Core asks several peers for tip-window blocks).
func (s *progressiveRawState) mayClaimContiguousHole(bs *BlockStoreCtx, workerID, workers int, lowMissing int64, inFlight map[int64][32]byte) bool {
	if lowMissing < 0 {
		return false
	}
	chunkLane := chunkLaneForWorker(workerID, workers)
	s.mu.Lock()
	lane0Alive := s.lane0AliveLocked()
	softOpen := s.softStallFrontier >= 0 && s.softStallFrontier == lowMissing
	staller := s.softStallPeer
	if staller == "" {
		staller = s.lastStallPeer
	}
	self := ""
	if s.laneAddr != nil {
		self = s.laneAddr[workerID]
	}
	lane0Active := false
	if s.laneDownloadSince != nil {
		if since, ok := s.laneDownloadSince[0]; ok && time.Since(since) < blockStallingTimeoutBodyIBD {
			lane0Active = true
		}
	}
	already := s.laneClaimsFrontierLocked(lowMissing, workerID)
	dupN := s.frontierClaimCountLocked(lowMissing)
	maxPeers := s.maxFrontierClaimPeersLocked(bs)
	s.mu.Unlock()
	if already {
		return false
	}
	if staller != "" && self != "" && self == staller {
		return false
	}
	_, busy := inFlight[lowMissing]
	if busy {
		return dupN < maxPeers
	}
	if softOpen {
		return true
	}
	if chunkLane == 0 {
		return true
	}
	if !lane0Alive || !lane0Active {
		return true
	}
	return false
}

func (s *progressiveRawState) frontierClaimCountLocked(h int64) int {
	if s == nil || h < 0 {
		return 0
	}
	n := 0
	if _, ok := s.inFlight[h]; ok {
		n = 1
	}
	if s.frontierExtraLanes != nil {
		n += len(s.frontierExtraLanes[h])
	}
	return n
}

func (s *progressiveRawState) laneClaimsFrontierLocked(h int64, lane int) bool {
	if s == nil || h < 0 || lane < 0 {
		return false
	}
	if s.inFlightLane != nil && s.inFlightLane[h] == lane {
		if _, ok := s.inFlight[h]; ok {
			return true
		}
	}
	if s.frontierExtraLanes == nil {
		return false
	}
	set, ok := s.frontierExtraLanes[h]
	if !ok || set == nil {
		return false
	}
	_, has := set[lane]
	return has
}

func (s *progressiveRawState) noteFrontierClaimLocked(h int64, lane int) {
	if s == nil || h < 0 || lane < 0 {
		return
	}
	if _, ok := s.inFlight[h]; !ok {
		return
	}
	if s.inFlightLane != nil && s.inFlightLane[h] == lane {
		return
	}
	if s.frontierExtraLanes == nil {
		s.frontierExtraLanes = make(map[int64]map[int]struct{})
	}
	set := s.frontierExtraLanes[h]
	if set == nil {
		set = make(map[int]struct{}, maxFrontierClaimPeers)
		s.frontierExtraLanes[h] = set
	}
	set[lane] = struct{}{}
}

func (s *progressiveRawState) clearFrontierClaimsLocked(h int64) {
	if s == nil || s.frontierExtraLanes == nil {
		return
	}
	delete(s.frontierExtraLanes, h)
}

// shouldUseParallelBatchChunks reports whether deep body IBD should assign each peer a
// disjoint ~getdata-sized height chunk (no duplicate asks) instead of collapsing all
// lanes onto the contiguous hole.
//
// When the contiguous frontier must be filled first (Core FindNextBlocksToDownload),
// exclusive chunks are a net loss: assist + relay lanes map onto the same chunkLane via
// modulo, fight over one stripe, and live IBD collapses into 1–3 block getdata scraps
// (~230 blk/min vs Core on the same host). Shared-window claiming with the global
// inFlight map already prevents duplicate asks.
func shouldUseParallelBatchChunks(bs *BlockStoreCtx, lowMissing int64) bool {
	if bs == nil || lowMissing < 0 {
		return false
	}
	if ConnectBodyGapHeight(bs) >= 0 {
		return false
	}
	if shouldFillContiguousFrontierFirst(bs, lowMissing) {
		return false
	}
	if bs.Journal == nil {
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
	// Near-tip / non-frontier cases only: keep exclusive chunks when striping helps.
	if gap > int64(blockDownloadWindow)/2 {
		return true
	}
	if cont < 32 {
		return false
	}
	return ShouldDeferConnectForBodyDownload(bs)
}

// syncBatchChunkBounds assigns lane workerID a disjoint getdata chunk of batchSize heights
// within [lowMissing, tip]. slotOffset selects the next round-robin chunk for that lane
// (0 = first chunk for this worker, 1 = workerID+workerCount, …) so a peer that finishes
// ~1000 blocks can immediately claim the next free 1000 without colliding with others.
func syncBatchChunkBounds(lowMissing, tip int64, workerID, workerCount, batchSize, slotOffset int) (lo, hi int64, ok bool) {
	if tip < 0 || lowMissing < 0 || lowMissing > tip || workerCount < 1 || batchSize < 1 {
		return 0, 0, false
	}
	if workerID < 0 || workerID >= workerCount {
		return 0, 0, false
	}
	if slotOffset < 0 {
		slotOffset = 0
	}
	slot := workerID + slotOffset*workerCount
	lo = lowMissing + int64(slot)*int64(batchSize)
	if lo > tip {
		return 0, 0, false
	}
	hi = lo + int64(batchSize) - 1
	if hi > tip {
		hi = tip
	}
	return lo, hi, true
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
