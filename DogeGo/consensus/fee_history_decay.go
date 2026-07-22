// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "math"

const defaultFeeHistoryHalflifeBlocks = 48.0

// EstimatePerKBDecay returns a feerate estimate with exponential decay by block age (Core-style recency bias).
func (h *FeeHistory) EstimatePerKBDecay(nblocks int) uint64 {
	return EstimateFeeFromRates(h.decayWeightedRatesForDepth(nblocks, defaultFeeHistoryHalflifeBlocks), nblocks)
}

func (h *FeeHistory) decayWeightedRatesForDepth(nblocks int, halflifeBlocks float64) []uint64 {
	if h == nil || nblocks <= 0 || halflifeBlocks <= 0 {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	depth := nblocks
	if depth > len(h.blocks) {
		depth = len(h.blocks)
	}
	var out []uint64
	for i := 0; i < depth; i++ {
		w := math.Pow(0.5, float64(i)/halflifeBlocks)
		repeats := int(w * 100)
		if repeats < 1 {
			repeats = 1
		}
		for _, r := range h.blocks[i] {
			for j := 0; j < repeats; j++ {
				out = append(out, r)
			}
		}
	}
	return out
}
