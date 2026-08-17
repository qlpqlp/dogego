// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "dogego/wire"

// CountBlockDustOutputs counts vouts below the effective dust threshold (non-coinbase txs only).
func CountBlockDustOutputs(pb *wire.ParsedBlock, pol StandardPolicy, dustRelayPerKB uint64) int64 {
	if pb == nil {
		return 0
	}
	return countBlockDustFromTxs(pb.Txs, pol, dustRelayPerKB)
}

// CountBlockDustOutputsRaw is CountBlockDustOutputs on serialized block bytes (no ParseBlock).
func CountBlockDustOutputsRaw(blockRaw []byte, pol StandardPolicy, dustRelayPerKB uint64) int64 {
	if len(blockRaw) < 80 {
		return 0
	}
	var n int64
	_ = wire.ForEachBlockTx(blockRaw, func(i uint32, tx *wire.Tx) error {
		if i == 0 || IsCoinbaseTx(tx) {
			return nil
		}
		for j := range tx.Vout {
			if IsOutputDustEffective(tx.Vout[j], pol, dustRelayPerKB) {
				n++
			}
		}
		return nil
	})
	return n
}

func countBlockDustFromTxs(txs []*wire.Tx, pol StandardPolicy, dustRelayPerKB uint64) int64 {
	var n int64
	for _, tx := range txs {
		if IsCoinbaseTx(tx) {
			continue
		}
		for i := range tx.Vout {
			if IsOutputDustEffective(tx.Vout[i], pol, dustRelayPerKB) {
				n++
			}
		}
	}
	return n
}

// BlockUtxoSizeIncrease estimates net serialized UTXO set growth for a block when parent prevouts resolve.
// Uses 8-byte value + scriptPubKey per output; subtracts spent prevout script sizes (Core undo analogue).
func BlockUtxoSizeIncrease(pb *wire.ParsedBlock, view PrevOutView) (int64, bool) {
	if pb == nil || view == nil {
		return 0, false
	}
	return blockUtxoSizeIncreaseFromTxs(pb.Txs, view)
}

// BlockUtxoSizeIncreaseRaw is BlockUtxoSizeIncrease on serialized block bytes (no ParseBlock).
func BlockUtxoSizeIncreaseRaw(blockRaw []byte, view PrevOutView) (int64, bool) {
	if len(blockRaw) < 80 || view == nil {
		return 0, false
	}
	var txs []*wire.Tx
	err := wire.ForEachBlockTx(blockRaw, func(i uint32, tx *wire.Tx) error {
		txs = append(txs, tx)
		return nil
	})
	if err != nil {
		return 0, false
	}
	return blockUtxoSizeIncreaseFromTxs(txs, view)
}

func blockUtxoSizeIncreaseFromTxs(txs []*wire.Tx, view PrevOutView) (int64, bool) {
	if len(txs) == 0 || view == nil {
		return 0, false
	}
	intra := &blockUndoView{}
	v := MultiPrevOutView{intra, view}
	var inc int64
	var resolved int
	for _, tx := range txs {
		if !IsCoinbaseTx(tx) {
			for _, in := range tx.Vin {
				if IsNullOutpoint(&in) {
					continue
				}
				if po, ok := v.Lookup(in.PrevHash, in.PrevIdx); ok {
					inc -= 8 + int64(len(po.PkScript))
					resolved++
				}
			}
		}
		for _, o := range tx.Vout {
			inc += 8 + int64(len(o.PkScript))
		}
		intra.addTx(tx, 0)
	}
	if resolved == 0 && len(txs) > 1 {
		return 0, false
	}
	return inc, true
}
