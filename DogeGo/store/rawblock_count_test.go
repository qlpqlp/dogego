// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"testing"

	"dogego/pow"
)

func TestReconcileCountCacheFromDiskBundled(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	payload, id := TestMinimalBlock()
	if err := raw.Put(id, payload); err != nil {
		t.Fatal(err)
	}
	prev := append([]byte(nil), payload[:80]...)
	for i := 1; i <= 3; i++ {
		h80 := append([]byte(nil), prev...)
		ph := pow.BlockHashLE(prev)
		copy(h80[4:36], ph[:])
		h80[76] ^= byte(i)
		next := MakeTestBlockRaw(t, h80)
		id := pow.BlockHashLE(next[:80])
		if err := raw.Put(id, next); err != nil {
			t.Fatal(err)
		}
		prev = append([]byte(nil), next[:80]...)
	}
	// Simulate restart: uninitialized counter; bundled FastCount probes blk*.dat once.
	raw.fileCount.Store(-1)
	if n, _ := raw.FastCount(); n != 4 {
		t.Fatalf("bundled FastCount after stale cache: %d want 4", n)
	}
	raw.fileCount.Store(99)
	if n, _ := raw.FastCount(); n != 99 {
		t.Fatalf("bundled FastCount must use cache, not rescan blk*.dat: %d", n)
	}
	raw.fileCount.Store(0)
	raw.ReconcileCountCacheFromDisk()
	if n, _ := raw.FastCount(); n != 4 {
		t.Fatalf("after bundled reconcile: %d want 4", n)
	}
}

func TestReconcileCountCacheFromDisk(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	h, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	rawBody := MakeTestBlockRaw(t, h[:])
	hash := pow.BlockHashLE(rawBody[:80])
	if err := raw.Put(hash, rawBody); err != nil {
		t.Fatal(err)
	}
	if n, _ := raw.FastCount(); n != 1 {
		t.Fatalf("after put: count %d want 1", n)
	}
	// Simulate stale cache (files on disk without counter bumps).
	raw.fileCount.Store(-1)
	if n, _ := raw.FastCount(); n != 1 {
		t.Fatalf("uninitialized FastCount should rescan: %d want 1", n)
	}
	raw.fileCount.Store(0)
	if n, _ := raw.FastCount(); n != 0 {
		t.Fatalf("stale cache: %d want 0", n)
	}
	raw.ReconcileCountCacheFromDisk()
	if n, _ := raw.FastCount(); n != 1 {
		t.Fatalf("after reconcile: %d want 1", n)
	}
}
