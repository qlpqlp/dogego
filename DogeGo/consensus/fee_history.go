// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"sort"
	"strconv"
	"sync"
)

const defaultFeeHistoryBlocks = 288

// StandardFeeBucketTargets are common estimatesmartfee confirmation horizons (blocks).
var StandardFeeBucketTargets = []int{1, 2, 3, 6, 12, 24, 48, 144}

// FeeHistory records confirmed feerates from connected blocks (lightweight Core estimator hint).
type FeeHistory struct {
	mu               sync.Mutex
	max              int
	blocks           [][]uint64       // newest block first (all block txs)
	mempoolConfirmed [][]uint64       // newest block first (txs that were in our mempool)
	confirmByTarget  map[int][]uint64 // blocks-waited bucket -> feerates from mempool confirms
	pendingMempool       map[string]pendingMempoolFee
	leftWithoutConfirm   [][]uint64       // feerates of txs that left mempool unconfirmed (newest first)
	leftByTarget         map[int][]uint64 // blocks-in-mempool horizon -> feerates when evicted/expired
	bucketMedians        map[int][]uint64 // target blocks -> recent per-block median feerates (koinu/kB)
	confirmStats              *TxConfirmStats  // exponential feerate buckets (Core TxConfirmStats)
	lastMempoolConfirmedTxIDs map[string]struct{} // txids counted via RecordMempoolConfirmedSamples (ConnectBlock dedupe)
}

// NewFeeHistory creates a tracker keeping up to maxBlocks recent confirmations (0 = default 288).
func NewFeeHistory(maxBlocks int) *FeeHistory {
	if maxBlocks <= 0 {
		maxBlocks = defaultFeeHistoryBlocks
	}
	return &FeeHistory{max: maxBlocks, confirmStats: NewTxConfirmStats()}
}

// NotifyBlockHeight advances fee-estimator block state (Core nBestSeenHeight + ClearCurrent).
func (h *FeeHistory) NotifyBlockHeight(height int64) {
	if h == nil || height < 0 {
		return
	}
	h.mu.Lock()
	if h.confirmStats != nil {
		h.confirmStats.AdvanceBlock(height)
	}
	h.mu.Unlock()
}

// Record stores feerates (koinu/kB) from one confirmed block.
func (h *FeeHistory) Record(rates []uint64) {
	if h == nil || len(rates) == 0 {
		return
	}
	cp := append([]uint64(nil), rates...)
	h.mu.Lock()
	h.blocks = append([][]uint64{cp}, h.blocks...)
	if len(h.blocks) > h.max {
		h.blocks = h.blocks[:h.max]
	}
	if med := medianUint64(cp); med > 0 {
		h.recordBucketMedianLocked(med)
	}
	h.mu.Unlock()
}

// RecordMempoolConfirmed stores feerates from txs that were in our mempool when a block arrived.
func (h *FeeHistory) RecordMempoolConfirmed(rates []uint64) {
	if h == nil || len(rates) == 0 {
		return
	}
	cp := append([]uint64(nil), rates...)
	h.mu.Lock()
	h.mempoolConfirmed = append([][]uint64{cp}, h.mempoolConfirmed...)
	if len(h.mempoolConfirmed) > h.max {
		h.mempoolConfirmed = h.mempoolConfirmed[:h.max]
	}
	h.mu.Unlock()
}

// RecordMempoolConfirmedSamples records feerates and blocks-to-confirm buckets (Core market hint).
func (h *FeeHistory) RecordMempoolConfirmedSamples(samples []MempoolConfirmSample) {
	if h == nil || len(samples) == 0 {
		return
	}
	rates := make([]uint64, 0, len(samples))
	for _, s := range samples {
		if s.FeeratePerKB > 0 {
			rates = append(rates, s.FeeratePerKB)
		}
	}
	h.RecordMempoolConfirmed(rates)
	h.mu.Lock()
	if h.confirmByTarget == nil {
		h.confirmByTarget = make(map[int][]uint64, len(StandardFeeBucketTargets))
	}
	for _, s := range samples {
		s = h.mergePendingConfirmSampleLocked(s)
		if s.FeeratePerKB == 0 || s.BlocksWaited <= 0 {
			continue
		}
		t := ClosestStandardBucketTarget(s.BlocksWaited)
		sl := append([]uint64{s.FeeratePerKB}, h.confirmByTarget[t]...)
		if len(sl) > h.max {
			sl = sl[:h.max]
		}
		h.confirmByTarget[t] = sl
		if h.confirmStats != nil {
			if s.TxID != "" {
				h.confirmStats.RemoveMempoolTx(s.TxID, h.confirmStats.bestSeenHeight)
			}
			h.confirmStats.RecordConfirm(s.BlocksWaited, s.FeeratePerKB)
		}
	}
	if h.confirmStats != nil {
		h.confirmStats.FlushBlock()
	}
	if len(samples) > 0 {
		h.lastMempoolConfirmedTxIDs = make(map[string]struct{}, len(samples))
		for _, s := range samples {
			if s.TxID != "" {
				h.lastMempoolConfirmedTxIDs[s.TxID] = struct{}{}
			}
		}
	} else {
		h.lastMempoolConfirmedTxIDs = nil
	}
	h.mu.Unlock()
}

func (h *FeeHistory) mergePendingConfirmSampleLocked(s MempoolConfirmSample) MempoolConfirmSample {
	if s.TxID != "" && h.pendingMempool != nil {
		if p, ok := h.pendingMempool[s.TxID]; ok {
			if p.feerate > 0 {
				s.FeeratePerKB = p.feerate
			}
			delete(h.pendingMempool, s.TxID)
		}
	}
	return s
}

func (h *FeeHistory) ratesMempoolConfirmedForDepth(nblocks int) []uint64 {
	if h == nil || nblocks <= 0 {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	depth := nblocks
	if depth > len(h.mempoolConfirmed) {
		depth = len(h.mempoolConfirmed)
	}
	var all []uint64
	for i := 0; i < depth; i++ {
		all = append(all, h.mempoolConfirmed[i]...)
	}
	return all
}

func (h *FeeHistory) ratesConfirmByTarget(nblocks int) []uint64 {
	if h == nil || nblocks <= 0 {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.confirmByTarget == nil {
		return nil
	}
	var all []uint64
	for _, bt := range StandardFeeBucketTargets {
		if bt <= nblocks {
			all = append(all, h.confirmByTarget[bt]...)
		}
	}
	return all
}

// EstimateMempoolConfirmedPerKB returns a feerate from recent mempool-confirmed txs (0 if none).
func (h *FeeHistory) EstimateMempoolConfirmedPerKB(nblocks int) uint64 {
	if rates := h.ratesConfirmByTarget(nblocks); len(rates) > 0 {
		if r := EstimateFeeFromRates(rates, nblocks); r > 0 {
			return r
		}
	}
	return EstimateFeeFromRates(h.ratesMempoolConfirmedForDepth(nblocks), nblocks)
}

// EstimateMempoolConfirmedPerKBEconomical returns a lower feerate from mempool-confirmed samples.
func (h *FeeHistory) EstimateMempoolConfirmedPerKBEconomical(nblocks int) uint64 {
	rates := h.ratesConfirmByTarget(nblocks)
	if len(rates) == 0 {
		rates = h.ratesMempoolConfirmedForDepth(nblocks)
	}
	return EstimateFeeFromRatesAtPercentile(rates, 0.25)
}

// MempoolConfirmBucketStats returns per-target stats from mempool-confirmed samples.
func (h *FeeHistory) MempoolConfirmBucketStats() map[string]map[string]interface{} {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.confirmByTarget) == 0 {
		return nil
	}
	out := make(map[string]map[string]interface{}, len(StandardFeeBucketTargets))
	for _, t := range StandardFeeBucketTargets {
		rates := h.confirmByTarget[t]
		if len(rates) == 0 {
			continue
		}
		key := strconv.Itoa(t)
		est := EstimateFeeFromRates(rates, t)
		out[key] = map[string]interface{}{
			"samples":            len(rates),
			"median_koinu_per_kb": medianUint64(rates),
			"median_doge_per_kb":  float64(medianUint64(rates)) / 1e8,
			"estimate_doge_per_kb": float64(est) / 1e8,
		}
	}
	return out
}

// MempoolLeftBucketStats returns per-target stats from txs that left the mempool unconfirmed.
func (h *FeeHistory) MempoolLeftBucketStats() map[string]map[string]interface{} {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.leftByTarget) == 0 {
		return nil
	}
	out := make(map[string]map[string]interface{}, len(StandardFeeBucketTargets))
	for _, t := range StandardFeeBucketTargets {
		rates := h.leftByTarget[t]
		if len(rates) == 0 {
			continue
		}
		key := strconv.Itoa(t)
		est := EstimateFeeFromRatesAtPercentile(rates, 1.0)
		out[key] = map[string]interface{}{
			"samples":              len(rates),
			"median_koinu_per_kb":  medianUint64(rates),
			"median_doge_per_kb":   float64(medianUint64(rates)) / 1e8,
			"estimate_doge_per_kb": float64(est) / 1e8,
		}
	}
	return out
}

func (h *FeeHistory) recordBucketMedianLocked(blockMedian uint64) {
	if h.bucketMedians == nil {
		h.bucketMedians = make(map[int][]uint64, len(StandardFeeBucketTargets))
	}
	for _, t := range StandardFeeBucketTargets {
		s := append([]uint64{blockMedian}, h.bucketMedians[t]...)
		if len(s) > h.max {
			s = s[:h.max]
		}
		h.bucketMedians[t] = s
	}
}

func medianUint64(v []uint64) uint64 {
	if len(v) == 0 {
		return 0
	}
	cp := append([]uint64(nil), v...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	return cp[len(cp)/2]
}

// ratesForDepth returns feerates from the newest depth blocks (Core bucket horizon analogue).
func (h *FeeHistory) ratesForDepth(nblocks int) []uint64 {
	if h == nil || nblocks <= 0 {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	depth := nblocks
	if depth > len(h.blocks) {
		depth = len(h.blocks)
	}
	var all []uint64
	for i := 0; i < depth; i++ {
		all = append(all, h.blocks[i]...)
	}
	return all
}

// EstimatePerKBFromBucketMedians returns a feerate from per-target block median history (Core bucket hint).
func (h *FeeHistory) EstimatePerKBFromBucketMedians(nblocks int) uint64 {
	if h == nil || nblocks <= 0 {
		return 0
	}
	t := ClosestStandardBucketTarget(nblocks)
	h.mu.Lock()
	meds := append([]uint64(nil), h.bucketMedians[t]...)
	h.mu.Unlock()
	return EstimateFeeFromRates(meds, t)
}

// EstimatePerKBEconomical returns the lowest viable feerate from recent confirmation history (Core economical mode hint).
func (h *FeeHistory) EstimatePerKBEconomical(nblocks int) uint64 {
	if h == nil || nblocks <= 0 {
		return 0
	}
	var candidates []uint64
	if r := h.EstimateMempoolConfirmedPerKBEconomical(nblocks); r > 0 {
		candidates = append(candidates, r)
	}
	if r := EstimateFeeFromRates(h.ratesForDepth(nblocks), nblocks); r > 0 {
		candidates = append(candidates, r)
	}
	if r := h.EstimatePerKBFromBucketMedians(nblocks); r > 0 {
		candidates = append(candidates, r)
	}
	if r := h.EstimatePerKBDecay(nblocks); r > 0 {
		candidates = append(candidates, r)
	}
	if r := h.EstimatePendingMempoolMinPerKB(); r > 0 {
		candidates = append(candidates, r)
	}
	if r, _ := h.EstimateConfirmStatsSmart(nblocks, false); r > 0 {
		candidates = append(candidates, r)
	}
	if min, ok := MinPositiveUint64(candidates...); ok {
		return min
	}
	return 0
}

// EstimateConfirmStatsSmart walks confirmation targets until a bucket estimate is found (Core estimateSmartFee).
// For targets above maxConfirmStatsConfirms (25), uses the capped stats plus ClosestStandardBucketTarget for answer blocks.
func (h *FeeHistory) EstimateConfirmStatsSmart(nblocks int, conservative bool) (uint64, int) {
	if h == nil || nblocks <= 0 {
		return 0, 0
	}
	start := nblocks
	if start == 1 {
		start = 2
	}
	if start > maxConfirmStatsConfirms {
		if r := h.EstimatePerKBFromConfirmStats(maxConfirmStatsConfirms, conservative); r > 0 {
			return r, ClosestStandardBucketTarget(nblocks)
		}
		return 0, 0
	}
	for t := start; t <= maxConfirmStatsConfirms; t++ {
		if r := h.EstimatePerKBFromConfirmStats(t, conservative); r > 0 {
			return r, t
		}
	}
	return 0, 0
}

// EstimatePerKBFromConfirmStats uses exponential feerate bucket confirmation stats (Core TxConfirmStats).
func (h *FeeHistory) EstimatePerKBFromConfirmStats(nblocks int, conservative bool) uint64 {
	if h == nil || nblocks <= 0 {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	stats := h.confirmStats
	if stats == nil {
		return 0
	}
	target := ClosestStandardBucketTarget(nblocks)
	if target > stats.maxConfirms {
		target = stats.maxConfirms
	}
	return stats.Estimate(target, conservative)
}

// ConfirmStatsBucketMarket returns per-target conservative/economical estimates from feerate buckets.
func (h *FeeHistory) ConfirmStatsBucketMarket() map[string]map[string]interface{} {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	stats := h.confirmStats
	if stats == nil {
		return nil
	}
	return stats.FeerateBucketMarketStats()
}

// EstimatePerKB returns a feerate from recent confirmations for an nblocks target (0 if empty).
func (h *FeeHistory) EstimatePerKB(nblocks int) uint64 {
	depth := EstimateFeeFromRates(h.ratesForDepth(nblocks), nblocks)
	bucket := h.EstimatePerKBFromBucketMedians(nblocks)
	if bucket > depth {
		return bucket
	}
	return depth
}

// EstimatePerKBHorizonMax returns the highest depth-scoped estimate from 1..nblocks (conservative market hint).
func (h *FeeHistory) EstimatePerKBHorizonMax(nblocks int) uint64 {
	if h == nil || nblocks <= 0 {
		return 0
	}
	var max uint64
	for d := 1; d <= nblocks; d++ {
		if r := h.EstimatePerKB(d); r > max {
			max = r
		}
	}
	return max
}

// EstimatePerKBConservative returns a high feerate from recent confirmations (Core conservative mode hint).
func (h *FeeHistory) EstimatePerKBConservative(nblocks int) uint64 {
	if h == nil || nblocks <= 0 {
		return 0
	}
	var max uint64
	for _, r := range []uint64{
		h.EstimatePerKBHorizonMax(nblocks),
		h.EstimatePerKBDecay(nblocks),
		h.EstimatePerKBFromBucketMedians(nblocks),
		h.EstimateMempoolConfirmedPerKB(nblocks),
		h.EstimateLeftWithoutConfirmPerKB(nblocks),
		EstimateFeeFromRatesAtPercentile(h.ratesForDepth(nblocks), 1.0),
		h.EstimatePerKB(nblocks),
	} {
		if r > max {
			max = r
		}
	}
	if cs, _ := h.EstimateConfirmStatsSmart(nblocks, true); cs > max {
		max = cs
	}
	return max
}

// ClosestStandardBucketTarget returns the nearest StandardFeeBucketTargets entry to nblocks.
func ClosestStandardBucketTarget(nblocks int) int {
	if nblocks <= 0 {
		return StandardFeeBucketTargets[0]
	}
	best := StandardFeeBucketTargets[0]
	bestDist := absInt(nblocks - best)
	for _, t := range StandardFeeBucketTargets[1:] {
		if d := absInt(nblocks - t); d < bestDist {
			best, bestDist = t, d
		}
	}
	return best
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// BucketMarketStats returns per-target market stats for RPC (samples + median + estimate DOGE/kB).
func (h *FeeHistory) BucketMarketStats() map[string]map[string]interface{} {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.bucketMedians) == 0 {
		return nil
	}
	out := make(map[string]map[string]interface{}, len(StandardFeeBucketTargets))
	for _, t := range StandardFeeBucketTargets {
		meds := h.bucketMedians[t]
		if len(meds) == 0 {
			continue
		}
		key := strconv.Itoa(t)
		est := EstimateFeeFromRates(meds, t)
		out[key] = map[string]interface{}{
			"samples":            len(meds),
			"median_koinu_per_kb": medianUint64(meds),
			"median_doge_per_kb":  float64(medianUint64(meds)) / 1e8,
			"estimate_doge_per_kb": float64(est) / 1e8,
		}
	}
	return out
}

// BucketEstimatesDOGE returns DOGE/kB estimates for StandardFeeBucketTargets.
func (h *FeeHistory) BucketEstimatesDOGE() map[string]float64 {
	if h == nil {
		return nil
	}
	out := make(map[string]float64, len(StandardFeeBucketTargets))
	for _, t := range StandardFeeBucketTargets {
		if r := h.EstimatePerKB(t); r > 0 {
			out[strconv.Itoa(t)] = float64(r) / 1e8
		}
	}
	return out
}
