// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
)

func TestMeasureContiguousBodiesOnDisk(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := MakeTestBlockRaw(t, g80[:])
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rbDir := filepath.Join(dir, "rawblocks")
	if err := os.MkdirAll(rbDir, 0o700); err != nil {
		t.Fatal(err)
	}
	genHash := pow.BlockHashLE(genesisRaw[:80])
	if err := os.WriteFile(filepath.Join(rbDir, hex.EncodeToString(genHash[:])+".bin"), genesisRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	prev := genesisRaw[:80]
	for i := 0; i < 5; i++ {
		h1 := append([]byte(nil), prev...)
		copy(h1[4:36], genHash[:])
		h1[76] ^= byte(i + 1)
		if err := j.AppendHeaders([][]byte{h1}); err != nil {
			t.Fatal(err)
		}
		body := append([]byte(nil), genesisRaw...)
		hHash := pow.BlockHashLE(h1)
		if err := os.WriteFile(filepath.Join(rbDir, hex.EncodeToString(hHash[:])+".bin"), body, 0o600); err != nil {
			t.Fatal(err)
		}
		prev = h1
		genHash = hHash
	}
	got := MeasureContiguousBodiesOnDisk(j, raw, chain.RebootTestnet, 0, 0)
	if got != 5 {
		t.Fatalf("contiguous=%d want 5", got)
	}
	if r := ReconcileBundledContiguousTip(j, raw, chain.RebootTestnet); r != 5 {
		t.Fatalf("reconcile=%d want 5", r)
	}
}

// TestReconcileBundledContiguousTipMeasuredAboveProbe simulates per-file body ahead of bundled
// blk tail (block-assist / locator drift): conservative tip is the blk scan (probe).
func TestReconcileBundledContiguousTipMeasuredAboveProbe(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := MakeTestBlockRaw(t, g80[:])
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	genHash := pow.BlockHashLE(genesisRaw[:80])
	if err := raw.Put(genHash, genesisRaw); err != nil {
		t.Fatal(err)
	}
	prev := genesisRaw[:80]
	for i := 0; i < 2; i++ {
		h1 := append([]byte(nil), prev...)
		copy(h1[4:36], genHash[:])
		h1[76] ^= byte(i + 1)
		if err := j.AppendHeaders([][]byte{h1}); err != nil {
			t.Fatal(err)
		}
		body := MakeTestBlockRaw(t, h1)
		hHash := pow.BlockHashLE(h1)
		if err := raw.Put(hHash, body); err != nil {
			t.Fatal(err)
		}
		prev = h1
		genHash = hHash
	}
	probe, err := raw.ProbeBundledContiguousTip()
	if err != nil || probe != 2 {
		t.Fatalf("probe=%d err=%v want 2", probe, err)
	}
	h3 := append([]byte(nil), prev...)
	copy(h3[4:36], genHash[:])
	h3[76] ^= 0x33
	if err := j.AppendHeaders([][]byte{h3}); err != nil {
		t.Fatal(err)
	}
	body3 := MakeTestBlockRaw(t, h3)
	hash3 := pow.BlockHashLE(h3)
	rbDir := filepath.Join(dir, "rawblocks")
	if err := os.MkdirAll(rbDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rbDir, hex.EncodeToString(hash3[:])+".bin"), body3, 0o600); err != nil {
		t.Fatal(err)
	}
	measured := MeasureContiguousBodiesOnDisk(j, raw, chain.RebootTestnet, 0, 0)
	if measured != 3 {
		t.Fatalf("measured=%d want 3", measured)
	}
	reconciled := ReconcileBundledContiguousTip(j, raw, chain.RebootTestnet)
	if reconciled != probe {
		t.Fatalf("reconciled=%d want conservative probe=%d (measured=%d)", reconciled, probe, measured)
	}
}

// TestReconcileBundledContiguousTipMeasuredBelowProbe simulates bundled blk records without
// matching header journal (operator partial rewind): conservative tip is header/body measure.
func TestReconcileBundledContiguousTipMeasuredBelowProbe(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw, genID := TestMinimalBlock()
	if err := raw.Put(genID, genesisRaw); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		payload, id := TestMinimalBlock()
		h80 := append([]byte(nil), payload[:80]...)
		ph := pow.BlockHashLE(h80)
		copy(h80[4:36], ph[:])
		h80[76] ^= byte(i + 1)
		payload = MakeTestBlockRaw(t, h80)
		id = pow.BlockHashLE(payload[:80])
		if err := raw.Put(id, payload); err != nil {
			t.Fatal(err)
		}
	}
	probe, err := raw.ProbeBundledContiguousTip()
	if err != nil || probe != 2 {
		t.Fatalf("probe=%d err=%v want 2", probe, err)
	}
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	measured := MeasureContiguousBodiesOnDisk(j, raw, chain.RebootTestnet, 0, 0)
	if measured != 0 {
		t.Fatalf("measured=%d want 0 (journal genesis only)", measured)
	}
	reconciled := ReconcileBundledContiguousTip(j, raw, chain.RebootTestnet)
	if reconciled != measured {
		t.Fatalf("reconciled=%d want measured=%d (probe=%d)", reconciled, measured, probe)
	}
}
