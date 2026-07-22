// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"os"
	"testing"

	"dogego/pow"
)

func TestPickBundledAppendSlotAdvancesOffset(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	payload, id := TestMinimalBlock()
	if err := raw.Put(id, payload); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(bundledBlkPath(raw.Dir(), 0))
	if err != nil {
		t.Fatal(err)
	}
	raw.mu.Lock()
	fileNum, off, err := raw.pickBundledAppendSlot(200)
	raw.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if fileNum != 0 || off != fi.Size() {
		t.Fatalf("fileNum=%d off=%d want off=%d", fileNum, off, fi.Size())
	}
	// second put should extend file
	h80 := append([]byte(nil), payload[:80]...)
	ph := pow.BlockHashLE(h80)
	copy(h80[4:36], ph[:])
	h80[76] ^= 1
	next := MakeTestBlockRaw(t, h80)
	if err := raw.Put(pow.BlockHashLE(next[:80]), next); err != nil {
		t.Fatal(err)
	}
	fi2, err := os.Stat(bundledBlkPath(raw.Dir(), 0))
	if err != nil {
		t.Fatal(err)
	}
	if fi2.Size() <= fi.Size() {
		t.Fatalf("file size %d want > %d after second put", fi2.Size(), fi.Size())
	}
}
