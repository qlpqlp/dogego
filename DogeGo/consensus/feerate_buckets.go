// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "strconv"

// Core policy/fees.h defaults (Dogecoin koinu per kB buckets).
const (
	minFeerateBucketKoinuPerKB = KoinuPerCoin / 1000 // 100_000
	maxFeerateBucketKoinuPerKB = KoinuPerCoin * 10   // 1_000_000_000
	feeBucketSpacing           = 1.05
	maxConfirmStatsConfirms    = 25
	defaultConfirmStatsDecay   = 1.0 - (0.693 / 60.0) // Core DEFAULT_DECAY
	minConfirmSuccessPct       = 0.8
	sufficientFeetxsPerBlock   = 0.1 // Core SUFFICIENT_FEETXS
)

// DefaultFeerateBucketUpperBounds returns exponentially spaced feerate bucket caps (koinu/kB).
func DefaultFeerateBucketUpperBounds() []uint64 {
	var out []uint64
	v := float64(minFeerateBucketKoinuPerKB)
	for v < float64(maxFeerateBucketKoinuPerKB) {
		out = append(out, uint64(v))
		v *= feeBucketSpacing
	}
	out = append(out, maxFeerateBucketKoinuPerKB)
	return out
}

func feerateBucketIndex(upperBounds []uint64, rate uint64) int {
	if len(upperBounds) == 0 || rate == 0 {
		return 0
	}
	for i, ub := range upperBounds {
		if rate <= ub {
			return i
		}
	}
	return len(upperBounds) - 1
}

type mempoolFeeTrack struct {
	bucketIndex  int
	entryHeight  int64
}

// TxConfirmStats tracks confirmation delay vs feerate bucket (Core TxConfirmStats subset).
type TxConfirmStats struct {
	decay          float64
	maxConfirms    int
	buckets        []uint64
	confAvg        [][]float64 // [confirmTarget-1][bucket]
	txCtAvg        []float64
	avg            []float64
	curConf        [][]float64
	curTxCt        []float64
	curVal         []float64
	bestSeenHeight int64
	unconfRing     [][]float64 // [height % maxConfirms][bucket]
	oldUnconf      []float64
	mempoolTxs     map[string]mempoolFeeTrack
}

// NewTxConfirmStats creates bucket stats with Core default spacing and decay.
func NewTxConfirmStats() *TxConfirmStats {
	bounds := DefaultFeerateBucketUpperBounds()
	return newTxConfirmStats(bounds, maxConfirmStatsConfirms, defaultConfirmStatsDecay)
}

func newTxConfirmStats(bounds []uint64, maxConfirms int, decay float64) *TxConfirmStats {
	if maxConfirms <= 0 {
		maxConfirms = maxConfirmStatsConfirms
	}
	if decay <= 0 || decay >= 1 {
		decay = defaultConfirmStatsDecay
	}
	nB := len(bounds)
	s := &TxConfirmStats{
		decay:       decay,
		maxConfirms: maxConfirms,
		buckets:     append([]uint64(nil), bounds...),
		txCtAvg:     make([]float64, nB),
		avg:         make([]float64, nB),
		curTxCt:     make([]float64, nB),
		curVal:      make([]float64, nB),
	}
	s.confAvg = make([][]float64, maxConfirms)
	s.curConf = make([][]float64, maxConfirms)
	s.unconfRing = make([][]float64, maxConfirms)
	for i := range s.confAvg {
		s.confAvg[i] = make([]float64, nB)
		s.curConf[i] = make([]float64, nB)
		s.unconfRing[i] = make([]float64, nB)
	}
	s.oldUnconf = make([]float64, nB)
	s.mempoolTxs = make(map[string]mempoolFeeTrack, 256)
	return s
}

// SetBestSeenHeight updates the chain tip used for mempool tx tracking (Core nBestSeenHeight).
func (s *TxConfirmStats) SetBestSeenHeight(height int64) {
	if s == nil || height < 0 {
		return
	}
	s.bestSeenHeight = height
}

// AdvanceBlock rolls unconfirmed bucket counters for a new block height (Core ClearCurrent).
func (s *TxConfirmStats) AdvanceBlock(height int64) {
	if s == nil || height < 0 {
		return
	}
	s.bestSeenHeight = height
	if len(s.unconfRing) == 0 {
		return
	}
	idx := int(height % int64(len(s.unconfRing)))
	for j := range s.buckets {
		s.oldUnconf[j] += s.unconfRing[idx][j]
		s.unconfRing[idx][j] = 0
	}
}

// TrackMempoolTx records an unconfirmed tx at the current best height (Core processTransaction).
func (s *TxConfirmStats) TrackMempoolTx(displayTxid string, entryHeight int64, feeratePerKB uint64) {
	if s == nil || displayTxid == "" || feeratePerKB == 0 || entryHeight < 0 {
		return
	}
	if entryHeight != s.bestSeenHeight {
		return
	}
	if _, ok := s.mempoolTxs[displayTxid]; ok {
		return
	}
	bi := feerateBucketIndex(s.buckets, feeratePerKB)
	if len(s.unconfRing) == 0 {
		return
	}
	blockIdx := int(entryHeight % int64(len(s.unconfRing)))
	s.unconfRing[blockIdx][bi]++
	s.mempoolTxs[displayTxid] = mempoolFeeTrack{bucketIndex: bi, entryHeight: entryHeight}
}

// RemoveMempoolTx stops tracking a mempool tx (confirm, evict, or expire).
func (s *TxConfirmStats) RemoveMempoolTx(displayTxid string, bestSeenHeight int64) {
	if s == nil || displayTxid == "" {
		return
	}
	tr, ok := s.mempoolTxs[displayTxid]
	if !ok {
		return
	}
	delete(s.mempoolTxs, displayTxid)
	s.removeMempoolBucket(tr, bestSeenHeight)
}

func (s *TxConfirmStats) removeMempoolBucket(tr mempoolFeeTrack, bestSeenHeight int64) {
	blocksAgo := int(bestSeenHeight - tr.entryHeight)
	if bestSeenHeight == 0 {
		blocksAgo = 0
	}
	if blocksAgo < 0 {
		return
	}
	bi := tr.bucketIndex
	if blocksAgo >= len(s.unconfRing) {
		if s.oldUnconf[bi] > 0 {
			s.oldUnconf[bi]--
		}
		return
	}
	blockIdx := int(tr.entryHeight % int64(len(s.unconfRing)))
	if s.unconfRing[blockIdx][bi] > 0 {
		s.unconfRing[blockIdx][bi]--
	}
}

// PendingMempoolTracks returns how many txs are tracked in feerate buckets.
func (s *TxConfirmStats) PendingMempoolTracks() int {
	if s == nil {
		return 0
	}
	return len(s.mempoolTxs)
}

// RecordConfirm records one mempool-confirmed tx (blocksToConfirm is 1-based).
func (s *TxConfirmStats) RecordConfirm(blocksToConfirm int, feeratePerKB uint64) {
	if s == nil || blocksToConfirm < 1 || feeratePerKB == 0 {
		return
	}
	bi := feerateBucketIndex(s.buckets, feeratePerKB)
	val := float64(feeratePerKB)
	for i := blocksToConfirm; i <= s.maxConfirms; i++ {
		s.curConf[i-1][bi]++
	}
	s.curTxCt[bi]++
	s.curVal[bi] += val
}

// FlushBlock applies decayed moving averages for the current block batch (Core UpdateMovingAverages).
func (s *TxConfirmStats) FlushBlock() {
	if s == nil {
		return
	}
	d := s.decay
	for j := range s.buckets {
		for i := range s.confAvg {
			s.confAvg[i][j] = s.confAvg[i][j]*d + s.curConf[i][j]
			s.curConf[i][j] = 0
		}
		s.avg[j] = s.avg[j]*d + s.curVal[j]
		s.txCtAvg[j] = s.txCtAvg[j]*d + s.curTxCt[j]
		s.curVal[j] = 0
		s.curTxCt[j] = 0
	}
}

// Estimate returns a feerate (koinu/kB) for confTarget blocks (Core conservative high-to-low bucket walk).
func (s *TxConfirmStats) Estimate(confTarget int, conservative bool) uint64 {
	if s == nil || confTarget < 1 {
		return 0
	}
	if !conservative {
		return s.estimateEconomical(confTarget)
	}
	return s.estimateConservative(confTarget)
}

func (s *TxConfirmStats) estimateConservative(confTarget int) uint64 {
	if confTarget > s.maxConfirms {
		confTarget = s.maxConfirms
	}
	sufficient := sufficientFeetxsPerBlock / (1 - s.decay)
	maxIdx := len(s.buckets) - 1

	var nConf, totalNum float64
	found := false
	bestNear, bestFar := maxIdx, maxIdx
	curNear := maxIdx

	for bucket := maxIdx; bucket >= 0; bucket-- {
		bestFar = bucket
		nConf += s.confAvg[confTarget-1][bucket]
		totalNum += s.txCtAvg[bucket]
		extra := s.unconfExtraForTarget(confTarget, bucket)
		if totalNum+extra >= sufficient {
			curPct := nConf / (totalNum + extra)
			if curPct < minConfirmSuccessPct {
				break
			}
			found = true
			nConf = 0
			totalNum = 0
			bestNear, bestFar = curNear, bucket
			curNear = bucket - 1
		}
	}
	if !found {
		return 0
	}
	minB, maxB := bestNear, bestFar
	if minB > maxB {
		minB, maxB = maxB, minB
	}
	return medianBucketFeerate(s, minB, maxB)
}

func (s *TxConfirmStats) unconfExtraForTarget(confTarget, bucket int) float64 {
	if s == nil || confTarget < 1 || bucket < 0 || bucket >= len(s.buckets) {
		return 0
	}
	var extra float64
	extra += s.oldUnconf[bucket]
	bins := len(s.unconfRing)
	if bins == 0 || s.bestSeenHeight <= 0 {
		return extra
	}
	for confct := confTarget; confct < s.maxConfirms; confct++ {
		idx := int((s.bestSeenHeight - int64(confct)) % int64(bins))
		if idx < 0 {
			idx += bins
		}
		extra += s.unconfRing[idx][bucket]
	}
	return extra
}

func (s *TxConfirmStats) estimateEconomical(confTarget int) uint64 {
	if confTarget > s.maxConfirms {
		confTarget = s.maxConfirms
	}
	sufficient := sufficientFeetxsPerBlock / (1 - s.decay)
	for bucket := 0; bucket < len(s.buckets); bucket++ {
		if s.txCtAvg[bucket] < sufficient {
			continue
		}
		if s.txCtAvg[bucket] == 0 {
			continue
		}
		if s.confAvg[confTarget-1][bucket]/s.txCtAvg[bucket] >= minConfirmSuccessPct {
			return uint64(s.avg[bucket] / s.txCtAvg[bucket])
		}
	}
	return 0
}

func medianBucketFeerate(s *TxConfirmStats, minB, maxB int) uint64 {
	var txSum float64
	for j := minB; j <= maxB; j++ {
		txSum += s.txCtAvg[j]
	}
	if txSum == 0 {
		return 0
	}
	txSum /= 2
	for j := minB; j <= maxB; j++ {
		if s.txCtAvg[j] < txSum {
			txSum -= s.txCtAvg[j]
			continue
		}
		if s.txCtAvg[j] > 0 {
			return uint64(s.avg[j] / s.txCtAvg[j])
		}
		break
	}
	return 0
}

// BucketCount returns the number of feerate buckets.
func (s *TxConfirmStats) BucketCount() int {
	if s == nil {
		return 0
	}
	return len(s.buckets)
}

// FeerateBucketMarketStats returns RPC-shaped per-target estimates from confirm stats.
func (s *TxConfirmStats) FeerateBucketMarketStats() map[string]map[string]interface{} {
	if s == nil {
		return nil
	}
	out := make(map[string]map[string]interface{}, len(StandardFeeBucketTargets))
	for _, t := range StandardFeeBucketTargets {
		ct := t
		if ct > s.maxConfirms {
			ct = s.maxConfirms
		}
		con := s.Estimate(ct, true)
		eco := s.Estimate(ct, false)
		if con == 0 && eco == 0 {
			continue
		}
		m := map[string]interface{}{
			"confirm_target": t,
		}
		if con > 0 {
			m["conservative_koinu_per_kb"] = con
			m["conservative_doge_per_kb"] = float64(con) / 1e8
		}
		if eco > 0 {
			m["economical_koinu_per_kb"] = eco
			m["economical_doge_per_kb"] = float64(eco) / 1e8
		}
		out[strconv.Itoa(t)] = m
	}
	return out
}
