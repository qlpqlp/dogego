// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store_test

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"dogego/pow"
	"dogego/store"
)

func TestWriteBehindHasAndGetBeforeDiskFlush(t *testing.T) {
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rs.EnableWriteBehind()
	t.Cleanup(func() { _ = rs.Flush() })
	raw, h := store.TestMinimalBlock()
	if err := rs.Put(h, raw); err != nil {
		t.Fatal(err)
	}
	if !rs.Has(h) {
		t.Fatal("Has must be true from RAM immediately")
	}
	if !rs.HasStoredBody(h, 80) {
		t.Fatal("HasStoredBody must treat RAM as stored so IBD contiguous/stall see the hole")
	}
	got, err := rs.Get(h)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatal("Get from RAM mismatch")
	}
	if err := rs.Flush(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rawblocks", hex.EncodeToString(h[:])+".bin")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("disk file after flush: %v", err)
	}
}

func TestWriteBehindParallelPutsAreReadable(t *testing.T) {
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rs.EnableWriteBehind()
	t.Cleanup(func() { _ = rs.Flush() })
	const n = 64
	hashes := make([][32]byte, n)
	for i := 0; i < n; i++ {
		raw, _ := store.TestMinimalBlock()
		raw[76] = byte(i)
		raw[77] = byte(i >> 8)
		h := pow.BlockHashLE(raw[:80])
		hashes[i] = h
		if err := rs.Put(h, raw); err != nil {
			t.Fatal(err)
		}
	}
	for i, h := range hashes {
		if !rs.HasStoredBody(h, 80) {
			t.Fatalf("height-equivalent %d missing from RAM", i)
		}
	}
	if err := rs.Flush(); err != nil {
		t.Fatal(err)
	}
}
