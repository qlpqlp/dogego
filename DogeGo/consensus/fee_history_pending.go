// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"dogego/mempool"
	"dogego/pow"
	"dogego/wire"
)

const maxPendingMempoolFeeTracks = 4096

func displayTxHash(h [32]byte) string {
	return pow.LEUint256DisplayHex(h[:])
}

type pendingMempoolFee struct {
	feerate uint64
	height  int64
}

// TrackMempoolAdmission records a tx accepted into our mempool for later confirmation stats.
func (h *FeeHistory) TrackMempoolAdmission(displayTxid string, feeratePerKB uint64, acceptHeight int64) {
	if h == nil || displayTxid == "" || feeratePerKB == 0 || acceptHeight < 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.pendingMempool == nil {
		h.pendingMempool = make(map[string]pendingMempoolFee, 128)
	}
	if len(h.pendingMempool) >= maxPendingMempoolFeeTracks {
		for k := range h.pendingMempool {
			delete(h.pendingMempool, k)
			break
		}
	}
	h.pendingMempool[displayTxid] = pendingMempoolFee{feerate: feeratePerKB, height: acceptHeight}
	if h.confirmStats != nil {
		h.confirmStats.TrackMempoolTx(displayTxid, acceptHeight, feeratePerKB)
	}
}

// UntrackMempoolTx drops a pending admission record without recording a failure (internal).
func (h *FeeHistory) UntrackMempoolTx(displayTxid string) {
	if h == nil || displayTxid == "" {
		return
	}
	h.mu.Lock()
	delete(h.pendingMempool, displayTxid)
	if h.confirmStats != nil {
		h.confirmStats.RemoveMempoolTx(displayTxid, h.confirmStats.bestSeenHeight)
	}
	h.mu.Unlock()
}

// RecordMempoolLeftWithoutConfirm records a tracked tx that left the mempool unconfirmed (Core failed estimate hint).
// It returns true when a pending track was found and a sample was stored.
func (h *FeeHistory) RecordMempoolLeftWithoutConfirm(displayTxid string, tipHeight int64) bool {
	if h == nil || displayTxid == "" || tipHeight < 0 {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	p, ok := h.pendingMempool[displayTxid]
	if !ok || p.feerate == 0 {
		delete(h.pendingMempool, displayTxid)
		return false
	}
	delete(h.pendingMempool, displayTxid)
	if h.confirmStats != nil {
		h.confirmStats.RemoveMempoolTx(displayTxid, tipHeight)
	}
	cp := []uint64{p.feerate}
	h.leftWithoutConfirm = append([][]uint64{cp}, h.leftWithoutConfirm...)
	if len(h.leftWithoutConfirm) > h.max {
		h.leftWithoutConfirm = h.leftWithoutConfirm[:h.max]
	}
	blocks := int(tipHeight - p.height + 1)
	if blocks < 1 {
		blocks = 1
	}
	if blocks > 144 {
		blocks = 144
	}
	if h.leftByTarget == nil {
		h.leftByTarget = make(map[int][]uint64, len(StandardFeeBucketTargets))
	}
	t := ClosestStandardBucketTarget(blocks)
	sl := append([]uint64{p.feerate}, h.leftByTarget[t]...)
	if len(sl) > h.max {
		sl = sl[:h.max]
	}
	h.leftByTarget[t] = sl
	return true
}

// LeftWithoutConfirmCount returns recent unconfirmed-leave events recorded.
func (h *FeeHistory) LeftWithoutConfirmCount() int {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	n := len(h.leftWithoutConfirm)
	h.mu.Unlock()
	return n
}

func (h *FeeHistory) ratesLeftWithoutConfirmForDepth(nblocks int) []uint64 {
	if h == nil || nblocks <= 0 {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	var all []uint64
	for _, bt := range StandardFeeBucketTargets {
		if bt <= nblocks {
			all = append(all, h.leftByTarget[bt]...)
		}
	}
	if len(all) == 0 {
		depth := nblocks
		if depth > len(h.leftWithoutConfirm) {
			depth = len(h.leftWithoutConfirm)
		}
		for i := 0; i < depth; i++ {
			all = append(all, h.leftWithoutConfirm[i]...)
		}
	}
	return all
}

// EstimateLeftWithoutConfirmPerKB returns a high feerate from txs that failed to confirm in time.
func (h *FeeHistory) EstimateLeftWithoutConfirmPerKB(nblocks int) uint64 {
	return EstimateFeeFromRatesAtPercentile(h.ratesLeftWithoutConfirmForDepth(nblocks), 1.0)
}

// EstimatePendingMempoolMinPerKB returns the lowest tracked admission feerate (economical mempool hint).
func (h *FeeHistory) EstimatePendingMempoolMinPerKB() uint64 {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	var min uint64
	for _, p := range h.pendingMempool {
		if p.feerate == 0 {
			continue
		}
		if min == 0 || p.feerate < min {
			min = p.feerate
		}
	}
	return min
}

// PendingMempoolFeeTracks returns how many mempool txs are tracked awaiting confirmation.
func (h *FeeHistory) PendingMempoolFeeTracks() int {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	n := len(h.pendingMempool)
	h.mu.Unlock()
	return n
}

// ConfirmStatsPendingTracks returns mempool txs counted in feerate confirm buckets.
func (h *FeeHistory) ConfirmStatsPendingTracks() int {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	n := 0
	if h.confirmStats != nil {
		n = h.confirmStats.PendingMempoolTracks()
	}
	h.mu.Unlock()
	return n
}

// ApplyLoadedPendingTracks restores TxConfirmStats unconf counters for pending txs at tip (Core mapMemPoolTxs).
// Stale entries older than maxConfirms below tip are dropped.
func (h *FeeHistory) ApplyLoadedPendingTracks(tipHeight int64) int {
	if h == nil || tipHeight < 0 {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.pendingMempool) == 0 {
		return 0
	}
	maxStale := maxConfirmStatsConfirms
	if h.confirmStats != nil && h.confirmStats.maxConfirms > 0 {
		maxStale = h.confirmStats.maxConfirms
	}
	cutoff := tipHeight - int64(maxStale)
	if h.confirmStats != nil && h.confirmStats.bestSeenHeight >= 0 {
		h.confirmStats.SetBestSeenHeight(tipHeight)
	}
	n := 0
	for id, p := range h.pendingMempool {
		if p.feerate == 0 {
			delete(h.pendingMempool, id)
			continue
		}
		if p.height >= 0 && p.height < cutoff {
			delete(h.pendingMempool, id)
			continue
		}
		if h.confirmStats == nil || p.height != tipHeight {
			continue
		}
		if _, ok := h.confirmStats.mempoolTxs[id]; ok {
			continue
		}
		if h.confirmStats.bestSeenHeight != tipHeight {
			continue
		}
		h.confirmStats.TrackMempoolTx(id, p.height, p.feerate)
		n++
	}
	return n
}

// RehydrateFromPool rebuilds pending feerate tracks after restart (Core mapMemPoolTxs restore analogue).
func (h *FeeHistory) RehydrateFromPool(pool *mempool.Pool, view PrevOutView, tipHeight int64) int {
	if h == nil || pool == nil || view == nil || tipHeight < 0 {
		return 0
	}
	txs, err := pool.SortedTransactions()
	if err != nil || len(txs) == 0 {
		return 0
	}
	n := 0
	for _, e := range txs {
		if e.TxID == "" || len(e.Raw) == 0 {
			continue
		}
		height := pool.EntryHeight(e.TxID)
		if height <= 0 {
			height = tipHeight
		}
		tx, derr := wire.DeserializeTx(e.Raw)
		if derr != nil {
			continue
		}
		rate, ok := TxFeeRateKoinuPerKB(tx, e.Raw, view)
		if !ok || rate == 0 {
			continue
		}
		h.mu.Lock()
		_, tracked := h.pendingMempool[e.TxID]
		h.mu.Unlock()
		if tracked {
			continue
		}
		h.TrackMempoolAdmission(e.TxID, rate, height)
		n++
	}
	return n
}

// TrackMempoolTxFee records admission feerate when prevouts resolve.
func TrackMempoolTxFee(h *FeeHistory, tx *wire.Tx, raw []byte, view PrevOutView, acceptHeight int64) {
	if h == nil || tx == nil || view == nil || acceptHeight < 0 {
		return
	}
	rate, ok := TxFeeRateKoinuPerKB(tx, raw, view)
	if !ok || rate == 0 {
		return
	}
	th := tx.TxHash()
	h.TrackMempoolAdmission(displayTxHash(th), rate, acceptHeight)
}
