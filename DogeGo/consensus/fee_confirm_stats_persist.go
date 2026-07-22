// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// confirmStatsUnconfFile is persisted TxConfirmStats unconf state (Core mapMemPoolTxs + ring).
type confirmStatsUnconfFile struct {
	UnconfRing    [][]float64                         `json:"unconf_ring,omitempty"`
	OldUnconf     []float64                           `json:"old_unconf,omitempty"`
	MempoolTracks map[string]confirmStatsTrackFile     `json:"mempool_tracks,omitempty"`
	BucketBounds  []uint64                            `json:"bucket_bounds,omitempty"`
}

type confirmStatsTrackFile struct {
	Bucket int   `json:"bucket"`
	Height int64 `json:"height"`
}

func (s *TxConfirmStats) hasPersistedState() bool {
	if s == nil {
		return false
	}
	if len(s.txCtAvg) > 0 {
		for _, v := range s.txCtAvg {
			if v > 0 {
				return true
			}
		}
	}
	if len(s.mempoolTxs) > 0 {
		return true
	}
	for _, v := range s.oldUnconf {
		if v > 0 {
			return true
		}
	}
	for _, row := range s.unconfRing {
		for _, v := range row {
			if v > 0 {
				return true
			}
		}
	}
	for _, row := range s.curConf {
		for _, v := range row {
			if v > 0 {
				return true
			}
		}
	}
	for _, v := range s.curTxCt {
		if v > 0 {
			return true
		}
	}
	for _, v := range s.curVal {
		if v > 0 {
			return true
		}
	}
	return false
}

func (s *TxConfirmStats) hasCurBatch() bool {
	if s == nil {
		return false
	}
	for _, row := range s.curConf {
		for _, v := range row {
			if v > 0 {
				return true
			}
		}
	}
	for _, v := range s.curTxCt {
		if v > 0 {
			return true
		}
	}
	for _, v := range s.curVal {
		if v > 0 {
			return true
		}
	}
	return false
}

func (s *TxConfirmStats) unconfSnapshot() *confirmStatsUnconfFile {
	if s == nil || !s.hasPersistedState() {
		return nil
	}
	out := &confirmStatsUnconfFile{
		UnconfRing:    clone2DFloat(s.unconfRing),
		OldUnconf:     append([]float64(nil), s.oldUnconf...),
		BucketBounds:  append([]uint64(nil), s.buckets...),
	}
	if len(s.mempoolTxs) > 0 {
		out.MempoolTracks = make(map[string]confirmStatsTrackFile, len(s.mempoolTxs))
		for id, tr := range s.mempoolTxs {
			out.MempoolTracks[id] = confirmStatsTrackFile{Bucket: tr.bucketIndex, Height: tr.entryHeight}
		}
	}
	return out
}

func applyUnconfSnapshot(stats *TxConfirmStats, snap *confirmStatsUnconfFile) {
	if stats == nil || snap == nil {
		return
	}
	if len(snap.BucketBounds) != len(stats.buckets) {
		return
	}
	for i, b := range snap.BucketBounds {
		if stats.buckets[i] != b {
			return
		}
	}
	if len(snap.OldUnconf) == len(stats.oldUnconf) {
		copy(stats.oldUnconf, snap.OldUnconf)
	}
	if len(snap.UnconfRing) == len(stats.unconfRing) {
		for i := range stats.unconfRing {
			if len(snap.UnconfRing[i]) == len(stats.unconfRing[i]) {
				copy(stats.unconfRing[i], snap.UnconfRing[i])
			}
		}
	}
	if len(snap.MempoolTracks) > 0 {
		if stats.mempoolTxs == nil {
			stats.mempoolTxs = make(map[string]mempoolFeeTrack, len(snap.MempoolTracks))
		}
		for id, tr := range snap.MempoolTracks {
			if tr.Bucket < 0 || tr.Bucket >= len(stats.buckets) {
				continue
			}
			stats.mempoolTxs[id] = mempoolFeeTrack{bucketIndex: tr.Bucket, entryHeight: tr.Height}
		}
	}
}

func applyCurBatchSnapshot(stats *TxConfirmStats, snap *confirmStatsFile) {
	if stats == nil || snap == nil {
		return
	}
	if len(snap.CurConf) == len(stats.curConf) {
		for i := range stats.curConf {
			if len(snap.CurConf[i]) == len(stats.curConf[i]) {
				copy(stats.curConf[i], snap.CurConf[i])
			}
		}
	}
	if len(snap.CurTxCt) == len(stats.curTxCt) {
		copy(stats.curTxCt, snap.CurTxCt)
	}
	if len(snap.CurVal) == len(stats.curVal) {
		copy(stats.curVal, snap.CurVal)
	}
}

// CatchUpBlockHeights advances confirm-stats block state from saved tip through tip (Core post-restart sync).
func (h *FeeHistory) CatchUpBlockHeights(tipHeight int64) {
	if h == nil || tipHeight < 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.confirmStats == nil {
		return
	}
	start := h.confirmStats.bestSeenHeight
	if tipHeight > start {
		for height := start + 1; height <= tipHeight; height++ {
			h.confirmStats.AdvanceBlock(height)
		}
	} else if tipHeight < start {
		h.confirmStats.SetBestSeenHeight(tipHeight)
	}
}
