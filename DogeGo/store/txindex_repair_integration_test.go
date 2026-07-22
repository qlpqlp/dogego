// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"os"
	"path/filepath"
	"testing"

	"dogego/pow"
)

// TestRepairTxIndexFromRawAfterIndexLoss simulates a partial tx index wipe and full repair from rawblocks.
func TestRepairTxIndexFromRawAfterIndexLoss(t *testing.T) {
	dir := t.TempDir()
	blockRaw, hash := TestMinimalBlock()
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), blockRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw.EnableTxIndexing(ix, true)
	if err := raw.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	prevID := hash
	for h := int64(1); h <= 5; h++ {
		prev, _ := j.ReadHeaderAt(h - 1)
		hdr := append([]byte(nil), prev...)
		copy(hdr[4:36], prevID[:])
		hdr[76] ^= byte(h)
		body := MakeTestBlockRaw(t, hdr)
		stored := append([]byte(nil), body[:80]...)
		id := pow.BlockHashLE(stored)
		if err := j.AppendHeaders([][]byte{stored}); err != nil {
			t.Fatal(err)
		}
		if err := raw.Put(id, body); err != nil {
			t.Fatal(err)
		}
		prevID = id
	}
	before, _, err := ix.Stats()
	if err != nil || before < 1 {
		t.Fatalf("indexed txs before wipe: %d err=%v", before, err)
	}

	entries, err := os.ReadDir(ix.RootDir())
	if err != nil {
		t.Fatal(err)
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(ix.RootDir(), e.Name())); err != nil {
			t.Fatal(err)
		}
		removed++
	}
	if removed == 0 {
		t.Fatal("expected tx files to remove")
	}

	rep, err := RepairTxIndexFromRaw(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rep.BlocksIndexed < 1 {
		t.Fatalf("repair blocks_indexed=%d", rep.BlocksIndexed)
	}
	after, _, err := ix.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if after < before {
		t.Fatalf("tx files after repair %d want >= %d", after, before)
	}
}
