// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"dogego/pow"
)

func TestRawBlockSyncCheckpointRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cp := RawBlockSyncCheckpoint{NextProbeHeight: 42}
	if err := SaveRawBlockSyncCheckpoint(dir, cp); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRawBlockSyncCheckpoint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.NextProbeHeight != 42 {
		t.Fatalf("got %d", got.NextProbeHeight)
	}
	cp2 := RawBlockSyncCheckpoint{NextProbeHeight: 100, ContiguousRawHeight: 2918}
	if err := SaveRawBlockSyncCheckpoint(dir, cp2); err != nil {
		t.Fatal(err)
	}
	got2, err := LoadRawBlockSyncCheckpoint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got2.ContiguousRawHeight != 2918 {
		t.Fatalf("contiguous=%d want 2918", got2.ContiguousRawHeight)
	}
	_, err = os.Stat(filepath.Join(dir, rawBlockSyncFile))
	if err != nil {
		t.Fatal(err)
	}
}

func TestPurgeStaleRawBlockSyncTemps(t *testing.T) {
	dir := t.TempDir()
	tmp := filepath.Join(dir, rawBlockSyncFile+".tmp")
	if err := os.WriteFile(tmp, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := PurgeStaleRawBlockSyncTemps(dir)
	if err != nil || n != 1 {
		t.Fatalf("purge n=%d err=%v", n, err)
	}
}

func TestReconcileRawBlockSyncCheckpointClampsStaleProbe(t *testing.T) {
	dir := t.TempDir()
	if err := SaveRawBlockSyncCheckpoint(dir, RawBlockSyncCheckpoint{NextProbeHeight: 800, ContiguousRawHeight: 750}); err != nil {
		t.Fatal(err)
	}
	fixed, err := ReconcileRawBlockSyncCheckpoint(dir, 200)
	if err != nil || !fixed {
		t.Fatalf("fixed=%v err=%v", fixed, err)
	}
	got, err := LoadRawBlockSyncCheckpoint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.NextProbeHeight != 201 || got.ContiguousRawHeight != 200 {
		t.Fatalf("got probe=%d cont=%d", got.NextProbeHeight, got.ContiguousRawHeight)
	}
}

func TestReconcileRawBlockSyncCheckpointCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, rawBlockSyncFile)
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixed, err := ReconcileRawBlockSyncCheckpoint(dir, 42)
	if err != nil || !fixed {
		t.Fatalf("fixed=%v err=%v", fixed, err)
	}
	got, err := LoadRawBlockSyncCheckpoint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.NextProbeHeight != 43 || got.ContiguousRawHeight != 42 {
		t.Fatalf("got probe=%d cont=%d", got.NextProbeHeight, got.ContiguousRawHeight)
	}
}

func TestProbeBundledContiguousTipReconcilesInflatedCheckpoint(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: true})
	if err != nil {
		t.Fatal(err)
	}
	payload, hash := TestMinimalBlock()
	if err := raw.Put(hash, payload); err != nil {
		t.Fatal(err)
	}
	tip, err := raw.ProbeBundledContiguousTip()
	if err != nil || tip != 0 {
		t.Fatalf("tip=%d err=%v", tip, err)
	}
	chainDir := dir
	if err := SaveRawBlockSyncCheckpoint(chainDir, RawBlockSyncCheckpoint{NextProbeHeight: 500, ContiguousRawHeight: 272}); err != nil {
		t.Fatal(err)
	}
	fixed, err := ReconcileRawBlockSyncCheckpoint(chainDir, tip)
	if err != nil || !fixed {
		t.Fatalf("fixed=%v err=%v", fixed, err)
	}
	got, err := LoadRawBlockSyncCheckpoint(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContiguousRawHeight != 0 || got.NextProbeHeight != 1 {
		t.Fatalf("got probe=%d cont=%d", got.NextProbeHeight, got.ContiguousRawHeight)
	}
}

// TestBundledTornTailReconcilesInflatedCheckpoint simulates kill mid-append: torn tail drops
// contiguous tip while rawblocks_sync.json still claims a higher frontier; reconcile clamps it.
func TestBundledTornTailReconcilesInflatedCheckpoint(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
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
	tip, err := raw.ProbeBundledContiguousTip()
	if err != nil || tip != 0 {
		t.Fatalf("after truncate tip=%d err=%v want 0", tip, err)
	}
	if err := SaveRawBlockSyncCheckpoint(dir, RawBlockSyncCheckpoint{NextProbeHeight: 500, ContiguousRawHeight: 272}); err != nil {
		t.Fatal(err)
	}
	fixed, err := ReconcileRawBlockSyncCheckpoint(dir, tip)
	if err != nil || !fixed {
		t.Fatalf("fixed=%v err=%v", fixed, err)
	}
	got, err := LoadRawBlockSyncCheckpoint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContiguousRawHeight != 0 || got.NextProbeHeight != 1 {
		t.Fatalf("got probe=%d cont=%d want 1/0", got.NextProbeHeight, got.ContiguousRawHeight)
	}
}
