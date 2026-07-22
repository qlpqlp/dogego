// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"dogego/mempool"
	"dogego/wire"
)

// BuildMempoolFeesKoinu returns per-tx fees in koinu for pooled transactions (omitted when unknown).
func BuildMempoolFeesKoinu(pool *mempool.Pool, view PrevOutView) map[string]int64 {
	out := make(map[string]int64)
	if pool == nil || view == nil {
		return out
	}
	ids, err := pool.RawMemPoolTxIDs()
	if err != nil {
		return out
	}
	for _, id := range ids {
		raw, err := pool.GetRawByTxID(id)
		if err != nil {
			continue
		}
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		fee, err := TxFee(tx, view)
		if err != nil || fee < 0 {
			continue
		}
		out[id] = fee
	}
	pool.ApplyFeeDeltas(out)
	return out
}

// MempoolEvictionMaps returns per-tx fees and sizes for ancestor-scored eviction.
func MempoolEvictionMaps(pool *mempool.Pool, view PrevOutView) (fees map[string]int64, sizes map[string]int) {
	fees = BuildMempoolFeesKoinu(pool, view)
	if pool != nil {
		sizes, _ = pool.BuildMempoolSizes()
	}
	return fees, sizes
}

// AddCandidateEvictionEntry adds fee/size for a tx about to enter the mempool (for feerate-aware eviction).
func AddCandidateEvictionEntry(tx *wire.Tx, raw []byte, view PrevOutView, fees map[string]int64, sizes map[string]int) {
	if tx == nil {
		return
	}
	id := txidDisplayFromLE(tx.TxHash())
	if sizes != nil {
		if _, ok := sizes[id]; !ok {
			if len(raw) > 0 {
				sizes[id] = len(raw)
			} else {
				sizes[id] = len(tx.SerializeForHash())
			}
		}
	}
	if fees != nil && view != nil {
		if _, ok := fees[id]; !ok {
			if fee, err := TxFee(tx, view); err == nil && fee >= 0 {
				fees[id] = fee
			}
		}
	}
}

// BuildMempoolFeeRates returns fee rate in koinu/kB for each pooled transaction (0 when unknown).
func BuildMempoolFeeRates(pool *mempool.Pool, view PrevOutView) mempool.FeeRateKoinuPerKB {
	out := make(mempool.FeeRateKoinuPerKB)
	if pool == nil || view == nil {
		return out
	}
	fees := BuildMempoolFeesKoinu(pool, view)
	for id, fee := range fees {
		raw, err := pool.GetRawByTxID(id)
		if err != nil {
			continue
		}
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		sz := len(tx.SerializeForHash())
		if sz <= 0 {
			continue
		}
		out[id] = fee * 1000 / int64(sz)
	}
	return out
}

// TotalMempoolFeesKoinu sums individual tx fees for pooled transactions with resolvable prevouts.
func TotalMempoolFeesKoinu(pool *mempool.Pool, view PrevOutView) int64 {
	if pool == nil || view == nil {
		return 0
	}
	var total int64
	ids, err := pool.RawMemPoolTxIDs()
	if err != nil {
		return 0
	}
	for _, id := range ids {
		raw, err := pool.GetRawByTxID(id)
		if err != nil {
			continue
		}
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		fee, err := TxFee(tx, view)
		if err != nil || fee < 0 {
			continue
		}
		total += fee
	}
	return total
}
