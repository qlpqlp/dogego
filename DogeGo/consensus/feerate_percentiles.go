// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "sort"

// StandardFeeratePercentiles are 10th, 25th, 50th, 75th, and 90th by cumulative vsize weight.
var StandardFeeratePercentiles = []float64{0.10, 0.25, 0.50, 0.75, 0.90}

// FeeratePercentilesKoinuPerKB returns feerate percentiles (koinu/kB) weighted by vsizes.
// rates and weights must align; empty input yields zeros.
func FeeratePercentilesKoinuPerKB(rates []uint64, weights []int) [5]uint64 {
	var out [5]uint64
	if len(rates) == 0 || len(weights) == 0 {
		return out
	}
	type sample struct {
		rate uint64
		w    int
	}
	var ps []sample
	var total int64
	for i := range rates {
		if rates[i] == 0 || i >= len(weights) || weights[i] <= 0 {
			continue
		}
		ps = append(ps, sample{rates[i], weights[i]})
		total += int64(weights[i])
	}
	if total == 0 || len(ps) == 0 {
		return out
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].rate < ps[j].rate })
	var cum int64
	ti := 0
	for _, p := range ps {
		cum += int64(p.w)
		for ti < len(StandardFeeratePercentiles) && float64(cum)/float64(total) >= StandardFeeratePercentiles[ti] {
			out[ti] = p.rate
			ti++
		}
		if ti >= len(out) {
			break
		}
	}
	last := ps[len(ps)-1].rate
	for i := ti; i < len(out); i++ {
		out[i] = last
	}
	return out
}
