// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "dogego/wire"

// BlockFeeSample is one non-coinbase tx feerate from a connected block.
type BlockFeeSample struct {
	TxID         string
	FeeratePerKB uint64
}

// BlockTxFeeSamplesRaw returns per-tx feerates by scanning serialized block bytes.
func BlockTxFeeSamplesRaw(blockRaw []byte, chainView PrevOutView) []BlockFeeSample {
	if len(blockRaw) < 80 || chainView == nil {
		return nil
	}
	intra := &blockUndoView{}
	view := MultiPrevOutView{intra, chainView}
	var out []BlockFeeSample
	_ = wire.ForEachBlockTx(blockRaw, func(i uint32, tx *wire.Tx) error {
		if i == 0 || IsCoinbaseTx(tx) {
			if IsCoinbaseTx(tx) {
				intra.addTx(tx, 0)
			}
			return nil
		}
		raw, _ := tx.Serialize()
		rate, ok := TxFeeRateKoinuPerKB(tx, raw, view)
		if ok && rate > 0 {
			out = append(out, BlockFeeSample{
				TxID:         displayTxHash(tx.TxHash()),
				FeeratePerKB: rate,
			})
		}
		intra.addTx(tx, 0)
		return nil
	})
	return out
}

// BlockTxFeeSamples returns per-tx feerates for non-coinbase txs (ConnectBlock / fee estimator).
func BlockTxFeeSamples(pb *wire.ParsedBlock, chainView PrevOutView) []BlockFeeSample {
	if pb == nil || chainView == nil {
		return nil
	}
	intra := &blockUndoView{}
	view := MultiPrevOutView{intra, chainView}
	var out []BlockFeeSample
	for i, tx := range pb.Txs {
		if i == 0 || IsCoinbaseTx(tx) {
			if IsCoinbaseTx(tx) {
				intra.addTx(tx, 0)
			}
			continue
		}
		raw, _ := tx.Serialize()
		rate, ok := TxFeeRateKoinuPerKB(tx, raw, view)
		if ok && rate > 0 {
			out = append(out, BlockFeeSample{
				TxID:         displayTxHash(tx.TxHash()),
				FeeratePerKB: rate,
			})
		}
		intra.addTx(tx, 0)
	}
	return out
}

// BlockTxFeeRates returns feerates (koinu/kB) for non-coinbase txs using the same prevout view as ConnectBlock.
func BlockTxFeeRates(pb *wire.ParsedBlock, chainView PrevOutView) []uint64 {
	samples := BlockTxFeeSamples(pb, chainView)
	if len(samples) == 0 {
		return nil
	}
	rates := make([]uint64, len(samples))
	for i, s := range samples {
		rates[i] = s.FeeratePerKB
	}
	return rates
}
