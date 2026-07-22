// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"sort"

	"dogego/wire"
)

// EstimateFeeFromRates picks a feerate (koinu/kB) from observed mempool rates for an nblocks target.
// Empty rates returns 0. Heuristic only - not Core's fee estimator buckets.
func EstimateFeeFromRates(rates []uint64, nblocks int) uint64 {
	if len(rates) == 0 {
		return 0
	}
	sort.Slice(rates, func(i, j int) bool { return rates[i] < rates[j] })
	idx := feeRatePercentileIndex(nblocks, len(rates))
	return rates[idx]
}

// EstimateFeeFromRatesMax returns the highest observed feerate (conservative mempool hint).
func EstimateFeeFromRatesMax(rates []uint64) uint64 {
	if len(rates) == 0 {
		return 0
	}
	sort.Slice(rates, func(i, j int) bool { return rates[i] < rates[j] })
	return rates[len(rates)-1]
}

// EstimateFeeFromRatesAtPercentile picks a feerate at the given percentile in [0,1].
func EstimateFeeFromRatesAtPercentile(rates []uint64, percentile float64) uint64 {
	if len(rates) == 0 {
		return 0
	}
	if percentile < 0 {
		percentile = 0
	}
	if percentile > 1 {
		percentile = 1
	}
	sort.Slice(rates, func(i, j int) bool { return rates[i] < rates[j] })
	idx := int(float64(len(rates)-1) * percentile)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(rates) {
		idx = len(rates) - 1
	}
	return rates[idx]
}

func feeRatePercentileIndex(nblocks, n int) int {
	if n <= 0 {
		return 0
	}
	pct := 0.75
	switch {
	case nblocks <= 2:
		pct = 1.0
	case nblocks <= 6:
		pct = 0.75
	case nblocks <= 24:
		pct = 0.5
	default:
		pct = 0.25
	}
	idx := int(float64(n-1) * pct)
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return idx
}

// CollectMempoolFeerateSamples returns paired feerates (koinu/kB) and vsizes for percentile stats.
func CollectMempoolFeerateSamples(rawTxs [][]byte, view PrevOutView) (rates []uint64, weights []int) {
	if view == nil {
		return nil, nil
	}
	for _, raw := range rawTxs {
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		rate, ok := TxFeeRateKoinuPerKB(tx, raw, view)
		if !ok || rate == 0 {
			continue
		}
		sz := len(raw)
		if sz <= 0 {
			continue
		}
		rates = append(rates, rate)
		weights = append(weights, sz)
	}
	return rates, weights
}

// CollectFeeRatesFromRaw returns feerates (koinu/kB) for serialized txs when prevouts resolve in view.
func CollectFeeRatesFromRaw(rawTxs [][]byte, view PrevOutView) []uint64 {
	if view == nil {
		return nil
	}
	var rates []uint64
	for _, raw := range rawTxs {
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		if rate, ok := TxFeeRateKoinuPerKB(tx, raw, view); ok && rate > 0 {
			rates = append(rates, rate)
		}
	}
	return rates
}

// EstimateMempoolFeePerKB combines mempool observation with nblocks target (0 when no priced txs).
func EstimateMempoolFeePerKB(rawTxs [][]byte, view PrevOutView, nblocks int) uint64 {
	return EstimateFeeFromRates(CollectFeeRatesFromRaw(rawTxs, view), nblocks)
}

// EstimateMempoolFeePerKBConservative uses the max observed mempool feerate (Core conservative hint).
func EstimateMempoolFeePerKBConservative(rawTxs [][]byte, view PrevOutView) uint64 {
	return EstimateFeeFromRatesMax(CollectFeeRatesFromRaw(rawTxs, view))
}

// EstimateMempoolFeePerKBEconomical uses a low mempool feerate percentile (Core economical hint).
func EstimateMempoolFeePerKBEconomical(rawTxs [][]byte, view PrevOutView, nblocks int) uint64 {
	rates := CollectFeeRatesFromRaw(rawTxs, view)
	if len(rates) == 0 {
		return 0
	}
	if r := EstimateFeeFromRatesAtPercentile(rates, 0.25); r > 0 {
		return r
	}
	return EstimateFeeFromRates(rates, nblocks)
}

// MinPositiveUint64 returns the smallest non-zero value, or (0, false) when none.
func MinPositiveUint64(vals ...uint64) (uint64, bool) {
	var min uint64
	var ok bool
	for _, v := range vals {
		if v == 0 {
			continue
		}
		if !ok || v < min {
			min, ok = v, true
		}
	}
	return min, ok
}
