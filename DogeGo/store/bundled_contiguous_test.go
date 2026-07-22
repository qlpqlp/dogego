// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/binary"
	"os"
	"testing"

	"dogego/pow"
)

// TestProbeBundledContiguousTipStopsAtTruncatedRecord ensures a torn tail record does not
// count as stored (Milestone B: bundled crash recovery semantics).
func TestProbeBundledContiguousTipStopsAtTruncatedRecord(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	var blocks [][]byte
	for i := 0; i < 3; i++ {
		payload, id := TestMinimalBlock()
		if i > 0 {
			h80 := append([]byte(nil), payload[:80]...)
			ph := pow.BlockHashLE(h80)
			copy(h80[4:36], ph[:])
			h80[76] ^= byte(i)
			payload = MakeTestBlockRaw(t, h80)
			id = pow.BlockHashLE(payload[:80])
		}
		if err := raw.Put(id, payload); err != nil {
			t.Fatal(err)
		}
		blocks = append(blocks, payload)
	}
	tip, err := raw.ProbeBundledContiguousTip()
	if err != nil || tip != 2 {
		t.Fatalf("tip=%d err=%v want 2", tip, err)
	}
	path := bundledBlkPath(raw.Dir(), 0)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < blockRecordHeaderLen*2 {
		t.Fatalf("file too small: %d", len(data))
	}
	if binary.LittleEndian.Uint32(data[0:]) != blockRecordMagic {
		t.Fatal("missing record magic")
	}
	storedLen := binary.LittleEndian.Uint32(data[8:12])
	firstRec := blockRecordHeaderLen + int(storedLen)
	if firstRec >= len(data) {
		t.Fatalf("firstRec=%d file=%d", firstRec, len(data))
	}
	cut := firstRec + 8
	if err := os.WriteFile(path, data[:cut], 0o600); err != nil {
		t.Fatal(err)
	}
	tip, err = raw.ProbeBundledContiguousTip()
	if err != nil || tip != 0 {
		t.Fatalf("after truncate tip=%d err=%v want 0", tip, err)
	}
	got, err := raw.GetByContiguousHeight(0)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(blocks[0]) {
		t.Fatal("height 0 mismatch after truncate")
	}
	if _, err := raw.GetByContiguousHeight(1); err == nil {
		t.Fatal("expected error for torn height 1")
	}
}

// TestBundledTornTailReopenConvergence ensures torn bundled tails survive store reopen
// (kill-and-restart semantics without manual repair).
func TestBundledTornTailReopenConvergence(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	var blocks [][]byte
	for i := 0; i < 3; i++ {
		payload, id := TestMinimalBlock()
		if i > 0 {
			h80 := append([]byte(nil), payload[:80]...)
			ph := pow.BlockHashLE(h80)
			copy(h80[4:36], ph[:])
			h80[76] ^= byte(i)
			payload = MakeTestBlockRaw(t, h80)
			id = pow.BlockHashLE(payload[:80])
		}
		if err := raw.Put(id, payload); err != nil {
			t.Fatal(err)
		}
		blocks = append(blocks, payload)
	}
	path := bundledBlkPath(raw.Dir(), 0)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	storedLen := binary.LittleEndian.Uint32(data[8:12])
	firstRec := blockRecordHeaderLen + int(storedLen)
	if err := os.WriteFile(path, data[:firstRec+8], 0o600); err != nil {
		t.Fatal(err)
	}
	raw2, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	tip, err := raw2.ProbeBundledContiguousTip()
	if err != nil || tip != 0 {
		t.Fatalf("after reopen tip=%d err=%v want 0", tip, err)
	}
	got, err := raw2.GetByContiguousHeight(0)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(blocks[0]) {
		t.Fatal("height 0 mismatch after reopen")
	}
	if _, err := raw2.GetByContiguousHeight(1); err == nil {
		t.Fatal("expected error for torn height 1 after reopen")
	}
}
