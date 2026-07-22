// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "dogego/wire"

// RecordBlockConnectedRaw records feerates from a serialized block without ParseBlock.
func (h *FeeHistory) RecordBlockConnectedRaw(blockRaw []byte, chainView PrevOutView) {
	if h == nil || len(blockRaw) < 80 || chainView == nil {
		return
	}
	h.recordBlockConnectedSamples(BlockTxFeeSamplesRaw(blockRaw, chainView))
}

// RecordBlockConnected stores block feerates and updates TxConfirmStats (Core processBlock / UpdateMovingAverages).
// Mempool-confirmed txids from RecordMempoolConfirmedSamples are not double-counted as 1-block confirms.
func (h *FeeHistory) RecordBlockConnected(pb *wire.ParsedBlock, chainView PrevOutView) {
	if h == nil || pb == nil || chainView == nil {
		return
	}
	h.recordBlockConnectedSamples(BlockTxFeeSamples(pb, chainView))
}

func (h *FeeHistory) recordBlockConnectedSamples(samples []BlockFeeSample) {
	if h == nil {
		return
	}
	if len(samples) == 0 {
		if h.confirmStats != nil {
			h.mu.Lock()
			h.confirmStats.FlushBlock()
			h.mu.Unlock()
		}
		return
	}
	rates := make([]uint64, len(samples))
	for i, s := range samples {
		rates[i] = s.FeeratePerKB
	}
	h.Record(rates)

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.confirmStats == nil {
		return
	}
	skip := h.lastMempoolConfirmedTxIDs
	for _, s := range samples {
		if s.FeeratePerKB == 0 {
			continue
		}
		if skip != nil {
			if _, ok := skip[s.TxID]; ok {
				continue
			}
		}
		h.confirmStats.RecordConfirm(1, s.FeeratePerKB)
	}
	h.confirmStats.FlushBlock()
	h.lastMempoolConfirmedTxIDs = nil
}

