// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	"dogego/store"
)

type feeHistoryFile struct {
	Blocks            [][]uint64          `json:"blocks"`
	MempoolConfirmed  [][]uint64          `json:"mempool_confirmed,omitempty"`
	ConfirmByTarget     map[string][]uint64 `json:"mempool_confirm_buckets,omitempty"`
	LeftWithoutConfirm  [][]uint64          `json:"left_without_confirm,omitempty"`
	LeftByTarget        map[string][]uint64 `json:"left_by_target,omitempty"`
	Buckets             map[string][]uint64 `json:"buckets,omitempty"` // target string -> recent block median feerates
	ConfirmStats        *confirmStatsFile   `json:"confirm_stats,omitempty"`
	PendingMempool      map[string]pendingMempoolFileEntry `json:"pending_mempool,omitempty"`
	BestSeenHeight      int64                              `json:"best_seen_height,omitempty"`
}

type pendingMempoolFileEntry struct {
	Feerate uint64 `json:"feerate"`
	Height  int64  `json:"height"`
}

type confirmStatsFile struct {
	Decay         float64                         `json:"decay"`
	ConfAvg       [][]float64                     `json:"conf_avg"`
	TxCtAvg       []float64                       `json:"tx_ct_avg"`
	Avg           []float64                       `json:"avg"`
	UnconfRing    [][]float64                     `json:"unconf_ring,omitempty"`
	OldUnconf     []float64                       `json:"old_unconf,omitempty"`
	MempoolTracks map[string]confirmStatsTrackFile `json:"mempool_tracks,omitempty"`
	BucketBounds  []uint64                        `json:"bucket_bounds,omitempty"`
	CurConf       [][]float64                     `json:"cur_conf,omitempty"`
	CurTxCt       []float64                       `json:"cur_tx_ct,omitempty"`
	CurVal        []float64                       `json:"cur_val,omitempty"`
}

// SaveFile writes confirmed feerate history to path (Core fee_estimates.dat analogue).
func (h *FeeHistory) SaveFile(path string) error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	snap := feeHistoryFile{Blocks: append([][]uint64(nil), h.blocks...)}
	if len(h.mempoolConfirmed) > 0 {
		snap.MempoolConfirmed = append([][]uint64(nil), h.mempoolConfirmed...)
	}
	if len(h.confirmByTarget) > 0 {
		snap.ConfirmByTarget = make(map[string][]uint64, len(h.confirmByTarget))
		for t, rates := range h.confirmByTarget {
			if len(rates) > 0 {
				snap.ConfirmByTarget[strconv.Itoa(t)] = append([]uint64(nil), rates...)
			}
		}
	}
	if len(h.leftWithoutConfirm) > 0 {
		snap.LeftWithoutConfirm = append([][]uint64(nil), h.leftWithoutConfirm...)
	}
	if len(h.leftByTarget) > 0 {
		snap.LeftByTarget = make(map[string][]uint64, len(h.leftByTarget))
		for t, rates := range h.leftByTarget {
			if len(rates) > 0 {
				snap.LeftByTarget[strconv.Itoa(t)] = append([]uint64(nil), rates...)
			}
		}
	}
	if len(h.bucketMedians) > 0 {
		snap.Buckets = make(map[string][]uint64, len(h.bucketMedians))
		for t, meds := range h.bucketMedians {
			if len(meds) > 0 {
				snap.Buckets[strconv.Itoa(t)] = append([]uint64(nil), meds...)
			}
		}
	}
	if h.confirmStats != nil && h.confirmStats.hasPersistedState() {
		snap.ConfirmStats = &confirmStatsFile{
			Decay:   h.confirmStats.decay,
			ConfAvg: clone2DFloat(h.confirmStats.confAvg),
			TxCtAvg: append([]float64(nil), h.confirmStats.txCtAvg...),
			Avg:     append([]float64(nil), h.confirmStats.avg...),
		}
		if u := h.confirmStats.unconfSnapshot(); u != nil {
			snap.ConfirmStats.UnconfRing = u.UnconfRing
			snap.ConfirmStats.OldUnconf = u.OldUnconf
			snap.ConfirmStats.MempoolTracks = u.MempoolTracks
			snap.ConfirmStats.BucketBounds = u.BucketBounds
		}
		if h.confirmStats.hasCurBatch() {
			snap.ConfirmStats.CurConf = clone2DFloat(h.confirmStats.curConf)
			snap.ConfirmStats.CurTxCt = append([]float64(nil), h.confirmStats.curTxCt...)
			snap.ConfirmStats.CurVal = append([]float64(nil), h.confirmStats.curVal...)
		}
	}
	if h.confirmStats != nil && h.confirmStats.bestSeenHeight >= 0 {
		snap.BestSeenHeight = h.confirmStats.bestSeenHeight
	}
	if len(h.pendingMempool) > 0 {
		snap.PendingMempool = make(map[string]pendingMempoolFileEntry, len(h.pendingMempool))
		for id, p := range h.pendingMempool {
			if p.feerate == 0 {
				continue
			}
			snap.PendingMempool[id] = pendingMempoolFileEntry{Feerate: p.feerate, Height: p.height}
		}
	}
	h.mu.Unlock()
	if len(snap.Blocks) == 0 && len(snap.MempoolConfirmed) == 0 && len(snap.ConfirmByTarget) == 0 &&
		len(snap.LeftWithoutConfirm) == 0 && len(snap.PendingMempool) == 0 && snap.BestSeenHeight == 0 {
		return nil
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	return store.AtomicWriteFile(path, raw, 0o600)
}

// LoadFeeHistoryFile restores history from a prior SaveFile (missing file returns nil, nil).
func LoadFeeHistoryFile(path string, maxBlocks int) (*FeeHistory, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var snap feeHistoryFile
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, err
	}
	h := NewFeeHistory(maxBlocks)
	h.mu.Lock()
	for _, blk := range snap.Blocks {
		if len(blk) == 0 {
			continue
		}
		h.blocks = append(h.blocks, append([]uint64(nil), blk...))
	}
	if len(h.blocks) > h.max {
		h.blocks = h.blocks[:h.max]
	}
	for _, blk := range snap.MempoolConfirmed {
		if len(blk) == 0 {
			continue
		}
		h.mempoolConfirmed = append(h.mempoolConfirmed, append([]uint64(nil), blk...))
	}
	if len(h.mempoolConfirmed) > h.max {
		h.mempoolConfirmed = h.mempoolConfirmed[:h.max]
	}
	if len(snap.ConfirmByTarget) > 0 {
		h.confirmByTarget = make(map[int][]uint64, len(snap.ConfirmByTarget))
		for key, rates := range snap.ConfirmByTarget {
			t, err := strconv.Atoi(key)
			if err != nil || len(rates) == 0 {
				continue
			}
			h.confirmByTarget[t] = append([]uint64(nil), rates...)
		}
	}
	for _, blk := range snap.LeftWithoutConfirm {
		if len(blk) > 0 {
			h.leftWithoutConfirm = append(h.leftWithoutConfirm, append([]uint64(nil), blk...))
		}
	}
	if len(h.leftWithoutConfirm) > h.max {
		h.leftWithoutConfirm = h.leftWithoutConfirm[:h.max]
	}
	if len(snap.LeftByTarget) > 0 {
		h.leftByTarget = make(map[int][]uint64, len(snap.LeftByTarget))
		for key, rates := range snap.LeftByTarget {
			t, err := strconv.Atoi(key)
			if err != nil || len(rates) == 0 {
				continue
			}
			h.leftByTarget[t] = append([]uint64(nil), rates...)
		}
	}
	if len(snap.Buckets) > 0 {
		h.bucketMedians = make(map[int][]uint64, len(snap.Buckets))
		for key, meds := range snap.Buckets {
			t, err := strconv.Atoi(key)
			if err != nil || len(meds) == 0 {
				continue
			}
			h.bucketMedians[t] = append([]uint64(nil), meds...)
		}
	}
	if snap.ConfirmStats != nil {
		stats := NewTxConfirmStats()
		if snap.ConfirmStats.Decay > 0 && snap.ConfirmStats.Decay < 1 {
			stats.decay = snap.ConfirmStats.Decay
		}
		if len(snap.ConfirmStats.ConfAvg) == len(stats.confAvg) {
			for i := range stats.confAvg {
				if len(snap.ConfirmStats.ConfAvg[i]) == len(stats.confAvg[i]) {
					copy(stats.confAvg[i], snap.ConfirmStats.ConfAvg[i])
				}
			}
		}
		if len(snap.ConfirmStats.TxCtAvg) == len(stats.txCtAvg) {
			copy(stats.txCtAvg, snap.ConfirmStats.TxCtAvg)
		}
		if len(snap.ConfirmStats.Avg) == len(stats.avg) {
			copy(stats.avg, snap.ConfirmStats.Avg)
		}
		applyUnconfSnapshot(stats, &confirmStatsUnconfFile{
			UnconfRing:    snap.ConfirmStats.UnconfRing,
			OldUnconf:     snap.ConfirmStats.OldUnconf,
			MempoolTracks: snap.ConfirmStats.MempoolTracks,
			BucketBounds:  snap.ConfirmStats.BucketBounds,
		})
		applyCurBatchSnapshot(stats, snap.ConfirmStats)
		h.confirmStats = stats
	}
	if snap.BestSeenHeight >= 0 && h.confirmStats != nil {
		h.confirmStats.SetBestSeenHeight(snap.BestSeenHeight)
	}
	if len(snap.PendingMempool) > 0 {
		h.pendingMempool = make(map[string]pendingMempoolFee, len(snap.PendingMempool))
		for id, p := range snap.PendingMempool {
			if p.Feerate == 0 {
				continue
			}
			h.pendingMempool[id] = pendingMempoolFee{feerate: p.Feerate, height: p.Height}
		}
	}
	h.mu.Unlock()
	return h, nil
}

func clone2DFloat(in [][]float64) [][]float64 {
	out := make([][]float64, len(in))
	for i := range in {
		out[i] = append([]float64(nil), in[i]...)
	}
	return out
}

// BlockCount returns how many confirmed blocks are in the history.
func (h *FeeHistory) BlockCount() int {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	n := len(h.blocks)
	h.mu.Unlock()
	return n
}
