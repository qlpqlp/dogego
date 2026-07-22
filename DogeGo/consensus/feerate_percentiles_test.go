// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestFeeratePercentilesKoinuPerKB(t *testing.T) {
	rates := []uint64{100, 200, 300, 400, 500}
	weights := []int{10, 10, 10, 10, 10}
	p := FeeratePercentilesKoinuPerKB(rates, weights)
	if p[0] != 100 || p[2] != 300 || p[4] != 500 {
		t.Fatalf("%v", p)
	}
}

func TestFeeratePercentilesKoinuPerKB_empty(t *testing.T) {
	p := FeeratePercentilesKoinuPerKB(nil, nil)
	if p[0] != 0 || p[4] != 0 {
		t.Fatalf("%v", p)
	}
}
