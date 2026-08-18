// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store_test

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"dogego/pow"
	"dogego/store"
)

func TestRawBlockStorePutHasCount(t *testing.T) {
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, h := store.TestMinimalBlock()
	if rs.Has(h) {
		t.Fatal("unexpected Has before Put")
	}
	if err := rs.Put(h, raw); err != nil {
		t.Fatal(err)
	}
	if !rs.Has(h) {
		t.Fatal("expected Has after Put")
	}
	if err := rs.Flush(); err != nil {
		t.Fatal(err)
	}
	n, err := rs.Count()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("count %d", n)
	}
	raw2, h2 := store.TestMinimalBlock()
	raw2[79] ^= 0x01
	h2 = pow.BlockHashLE(raw2[:80])
	if err := rs.Put(h2, raw2); err != nil {
		t.Fatal(err)
	}
	if err := rs.Flush(); err != nil {
		t.Fatal(err)
	}
	n2, _ := rs.Count()
	if n2 != 2 {
		t.Fatalf("count2 %d", n2)
	}
	if err := rs.Flush(); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "rawblocks", "*.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("glob want 2 files, got %d: %v", len(matches), matches)
	}
}

func TestRawBlockStoreParallelPerFilePut(t *testing.T) {
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	const n = 32
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			raw, _ := store.TestMinimalBlock()
			raw[76] = byte(i)
			raw[77] = byte(i >> 8)
			h := pow.BlockHashLE(raw[:80])
			errCh <- rs.Put(h, raw)
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := rs.Flush(); err != nil {
		t.Fatal(err)
	}
	count, err := rs.Count()
	if err != nil {
		t.Fatal(err)
	}
	if count != n {
		t.Fatalf("count %d want %d", count, n)
	}
}

func TestRawBlockStoreGet(t *testing.T) {
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, h := store.TestMinimalBlock()
	if err := rs.Put(h, raw); err != nil {
		t.Fatal(err)
	}
	got, err := rs.Get(h)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatalf("payload mismatch")
	}
	var missing [32]byte
	missing[1] = 1
	if _, err := rs.Get(missing); err == nil {
		t.Fatal("expected error for missing block")
	}
}

func TestRawBlockStoreBytesOnDisk(t *testing.T) {
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, h := store.TestMinimalBlock()
	if err := rs.Put(h, raw); err != nil {
		t.Fatal(err)
	}
	if err := rs.Flush(); err != nil {
		t.Fatal(err)
	}
	n, err := rs.BytesOnDisk()
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(raw)) {
		t.Fatalf("BytesOnDisk %d want %d", n, len(raw))
	}
}

func TestRawBlockStoreCachedBytesOnDisk(t *testing.T) {
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, h := store.TestMinimalBlock()
	if err := rs.Put(h, raw); err != nil {
		t.Fatal(err)
	}
	if err := rs.Flush(); err != nil {
		t.Fatal(err)
	}
	n1, err := rs.CachedBytesOnDisk(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	n2, err := rs.CachedBytesOnDisk(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if n1 != n2 || n1 != int64(len(raw)) {
		t.Fatalf("CachedBytesOnDisk %d %d want %d", n1, n2, len(raw))
	}
	rs.InvalidateBytesOnDiskCache()
	n3, err := rs.CachedBytesOnDisk(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if n3 != int64(len(raw)) {
		t.Fatalf("after invalidate CachedBytesOnDisk %d want %d", n3, len(raw))
	}
}

func TestRawBlockStorePutRejectsBadMerkle(t *testing.T) {
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, h := store.TestMinimalBlock()
	raw[36] ^= 0x01
	if err := rs.Put(h, raw); err == nil {
		t.Fatal("expected merkle rejection")
	}
}
