// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"dogego/consensus"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

func mempoolAdmissionView(pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore) consensus.PrevOutView {
	return consensus.AdmissionPrevOutView(pool, txIndex, blocks)
}

func mempoolFeeratePercentilesDOGE(pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore) []interface{} {
	view := mempoolAdmissionView(pool, txIndex, blocks)
	if view == nil || pool == nil {
		return nil
	}
	rates, weights := consensus.CollectMempoolFeerateSamples(pool.RawBlobs(), view)
	p := consensus.FeeratePercentilesKoinuPerKB(rates, weights)
	out := make([]interface{}, len(p))
	for i, k := range p {
		out[i] = float64(k) / 1e8
	}
	return out
}

func txFeeDOGE(tx *wire.Tx, view consensus.PrevOutView) (feeDOGE float64, feeRateDOGEPerKB float64) {
	if tx == nil || view == nil {
		return 0, 0
	}
	fee, err := consensus.TxFee(tx, view)
	if err != nil || fee < 0 {
		return 0, 0
	}
	sz := len(tx.SerializeForHash())
	if sz <= 0 {
		return float64(fee) / 1e8, 0
	}
	rateKoinuPerKB := float64(fee) * 1000 / float64(sz)
	return float64(fee) / 1e8, rateKoinuPerKB / 1e8
}
