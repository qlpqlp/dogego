// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"path/filepath"
	"testing"

	"dogego/pow"
)

// Stale cached tip ahead of headers.bin caused prune to read past EOF (height 3361, size 268880).
func TestPruneChainDataAboveHeight_staleAheadCache(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	h1 := append([]byte(nil), g80[:]...)
	h1[76] ^= 1
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	tip, err := j.TipHeight()
	if err != nil || tip != 1 {
		t.Fatalf("tip=%d err=%v", tip, err)
	}
	// Simulate incremental cache overshoot (one header on disk, tip cached as 2).
	j.cachedCount.Store(3)
	j.cachedTip.Store(2)
	_, err = PruneChainDataAboveHeight(j, nil, nil, 0)
	if err != nil {
		t.Fatalf("prune with stale cache: %v", err)
	}
	tip, err = j.TipHeight()
	if err != nil || tip != 1 {
		t.Fatalf("after reconcile tip=%d err=%v", tip, err)
	}
}
