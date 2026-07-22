// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestUtxoHeightFromViewNestedMulti(t *testing.T) {
	var prev [32]byte
	prev[2] = 0x11
	src := stubUtxoHeightSource{heights: map[[36]byte]int64{outpointKey(prev, 0): 99}}
	outer := MultiPrevOutView{&blockUndoView{}, UtxoPrevOutView{Source: src}}
	h, ok := utxoHeightFromView(outer, prev, 0)
	if !ok || h != 99 {
		t.Fatalf("height=%d ok=%v want 99 true", h, ok)
	}
}
