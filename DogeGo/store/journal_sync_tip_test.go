// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import "testing"

func TestSyncTipFromDiskAfterAppend(t *testing.T) {
	gen := make([]byte, 80)
	j, err := OpenHeaderJournal(t.TempDir()+"/headers.bin", gen)
	if err != nil {
		t.Fatal(err)
	}
	batch := make([]byte, 80*3)
	for i := 0; i < 3; i++ {
		copy(batch[i*80:(i+1)*80], gen)
	}
	if err := j.AppendWireHeaderBatch(batch); err != nil {
		t.Fatal(err)
	}
	j.cachedCount.Store(1)
	j.cachedTip.Store(0)

	tip, count, err := j.SyncTipFromDisk()
	if err != nil {
		t.Fatal(err)
	}
	if count != 4 || tip != 3 {
		t.Fatalf("SyncTipFromDisk tip=%d count=%d want 3/4", tip, count)
	}
}

func TestCountReconcilesStaleCache(t *testing.T) {
	gen := make([]byte, 80)
	j, err := OpenHeaderJournal(t.TempDir()+"/h.bin", gen)
	if err != nil {
		t.Fatal(err)
	}
	batch := make([]byte, 80*10)
	for i := 0; i < 10; i++ {
		copy(batch[i*80:(i+1)*80], gen)
	}
	if err := j.AppendWireHeaderBatch(batch); err != nil {
		t.Fatal(err)
	}
	j.cachedCount.Store(2)
	j.cachedTip.Store(1)
	c, err := j.Count()
	if err != nil {
		t.Fatal(err)
	}
	if c != 11 {
		t.Fatalf("Count=%d want 11 (genesis+10)", c)
	}
}

func TestSyncTipFromDiskSegmentReload(t *testing.T) {
	gen := make([]byte, 80)
	dir := t.TempDir()
	j, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	batch := make([]byte, 80*5)
	for i := 0; i < 5; i++ {
		copy(batch[i*80:(i+1)*80], gen)
	}
	if err := j.AppendWireHeaderBatch(batch); err != nil {
		t.Fatal(err)
	}
	j.seg.mu.Lock()
	j.seg.manifest.TipHeight = 0
	j.seg.mu.Unlock()
	j.cachedCount.Store(1)
	j.cachedTip.Store(0)

	tip, count, err := j.SyncTipFromDisk()
	if err != nil {
		t.Fatal(err)
	}
	if count != 6 || tip != 5 {
		t.Fatalf("SyncTipFromDisk segment tip=%d count=%d want 5/6", tip, count)
	}
}
