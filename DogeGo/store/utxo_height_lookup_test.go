// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"testing"
)

func TestUtxoCacheUnspentHeight(t *testing.T) {
	u := NewUtxoCache()
	var prev [32]byte
	prev[1] = 0x42
	k := outpointKey(prev, 1)
	u.mu.Lock()
	u.coins[k] = UtxoEntry{Value: 100, Height: 99, PkScript: []byte{0x76}}
	u.mu.Unlock()
	h, ok := u.UnspentHeight(prev, 1)
	if !ok || h != 99 {
		t.Fatalf("height=%d ok=%v want 99 true", h, ok)
	}
	if _, ok := u.UnspentHeight(prev, 0); ok {
		t.Fatal("missing outpoint should not resolve")
	}
}
