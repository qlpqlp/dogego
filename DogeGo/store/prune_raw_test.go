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

func TestPruneRawBlocksBelowHeight(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	blockRaw := makeTestBlockRaw(t, g80[:])
	h1 := blockRaw[:80]
	gen := pow.BlockHashLE(g80[:])
	copy(h1[4:36], gen[:])
	h1[76] ^= 1
	blockRaw = makeTestBlockRaw(t, h1)
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	id1 := pow.BlockHashLE(blockRaw[:80])
	if err := raw.Put(id1, blockRaw); err != nil {
		t.Fatal(err)
	}
	last, n, err := PruneRawBlocksBelowHeight(j, raw, nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	if last != 1 || n != 1 {
		t.Fatalf("last=%d n=%d", last, n)
	}
	if raw.Has(id1) {
		t.Fatal("height 1 raw should be pruned")
	}
}
