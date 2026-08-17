// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "dogego/wire"

// BlockFeeStats holds per-block fee aggregates for getblockstats (Core-shaped; feerates in koinu/byte).
type BlockFeeStats struct {
	TotalFee           int64
	MinFee, MaxFee     int64
	MedianFee          int64
	MinFeerate         int64 // koinu per serialized byte
	MaxFeerate         int64
	AvgFeerate         int64
	FeeratePercentiles [5]uint64
}

// ComputeBlockFeeStats sums non-coinbase tx fees when prevouts resolve in view (parent chain state).
func ComputeBlockFeeStats(pb *wire.ParsedBlock, view PrevOutView) (BlockFeeStats, bool) {
	if pb == nil || view == nil {
		return BlockFeeStats{}, false
	}
	return computeBlockFeeStatsFromTxs(pb.Txs, view)
}

// ComputeBlockFeeStatsRaw is ComputeBlockFeeStats on serialized block bytes (no ParseBlock).
func ComputeBlockFeeStatsRaw(blockRaw []byte, view PrevOutView) (BlockFeeStats, bool) {
	if len(blockRaw) < 80 || view == nil {
		return BlockFeeStats{}, false
	}
	var txs []*wire.Tx
	err := wire.ForEachBlockTx(blockRaw, func(i uint32, tx *wire.Tx) error {
		txs = append(txs, tx)
		return nil
	})
	if err != nil || len(txs) == 0 {
		return BlockFeeStats{}, false
	}
	return computeBlockFeeStatsFromTxs(txs, view)
}

func computeBlockFeeStatsFromTxs(txs []*wire.Tx, view PrevOutView) (BlockFeeStats, bool) {
	var out BlockFeeStats
	if len(txs) == 0 || view == nil {
		return out, false
	}
	intra := &blockUndoView{}
	v := MultiPrevOutView{intra, view}
	var fees []int64
	var sizes []int
	for i, tx := range txs {
		if i == 0 || IsCoinbaseTx(tx) {
			if IsCoinbaseTx(tx) {
				intra.addTx(tx, 0)
			}
			continue
		}
		raw, err := tx.Serialize()
		if err != nil {
			intra.addTx(tx, 0)
			continue
		}
		fee, err := TxFee(tx, v)
		if err != nil || fee < 0 {
			intra.addTx(tx, 0)
			continue
		}
		sz := len(raw)
		if sz <= 0 {
			intra.addTx(tx, 0)
			continue
		}
		fees = append(fees, fee)
		sizes = append(sizes, sz)
		out.TotalFee += fee
		rate := fee / int64(sz)
		if len(fees) == 1 {
			out.MinFee, out.MaxFee = fee, fee
			out.MinFeerate, out.MaxFeerate = rate, rate
		} else {
			if fee < out.MinFee {
				out.MinFee = fee
			}
			if fee > out.MaxFee {
				out.MaxFee = fee
			}
			if rate < out.MinFeerate {
				out.MinFeerate = rate
			}
			if rate > out.MaxFeerate {
				out.MaxFeerate = rate
			}
		}
		intra.addTx(tx, 0)
	}
	if len(fees) == 0 {
		return out, false
	}
	out.MedianFee = medianInt64(fees)
	var totalSize int64
	for _, s := range sizes {
		totalSize += int64(s)
	}
	if totalSize > 0 {
		out.AvgFeerate = out.TotalFee / totalSize
	}
	rates := make([]uint64, len(fees))
	for i := range fees {
		if sizes[i] > 0 {
			rates[i] = uint64(fees[i] / int64(sizes[i]))
		}
	}
	out.FeeratePercentiles = FeeratePercentilesKoinuPerKB(rates, sizes)
	return out, true
}

func medianInt64(vals []int64) int64 {
	n := len(vals)
	if n == 0 {
		return 0
	}
	cp := append([]int64(nil), vals...)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if cp[j] < cp[i] {
				cp[i], cp[j] = cp[j], cp[i]
			}
		}
	}
	if n%2 == 0 {
		return (cp[n/2-1] + cp[n/2]) / 2
	}
	return cp[n/2]
}
