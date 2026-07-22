// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"path/filepath"
	"testing"

	"dogego/pow"
	"dogego/store"
)

func TestInitFromCheckpointClampsAheadOfContiguous(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, genesisRaw[:80], 100)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	if err := store.SaveRawBlockSyncCheckpoint(dir, store.RawBlockSyncCheckpoint{NextProbeHeight: 80}); err != nil {
		t.Fatal(err)
	}
	var s progressiveRawState
	s.InitFromCheckpoint(dir, 100, 1)
	if s.nextProbe != 2 {
		t.Fatalf("nextProbe %d want 2 (contiguous 1 → fill from 2)", s.nextProbe)
	}
}

func TestInitFromCheckpointClampsStaleProbeWhenNoContiguousBodies(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, genesisRaw[:80], 100)
	if err := store.SaveRawBlockSyncCheckpoint(dir, store.RawBlockSyncCheckpoint{NextProbeHeight: 80}); err != nil {
		t.Fatal(err)
	}
	var s progressiveRawState
	s.InitFromCheckpoint(dir, 100, -1)
	if s.nextProbe != 0 {
		t.Fatalf("nextProbe %d want 0 when contiguous unknown and checkpoint ahead", s.nextProbe)
	}
}

func TestInitFromCheckpointGenesisHeight(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, genesisRaw[:80], 10)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRawBlockSyncCheckpoint(dir, store.RawBlockSyncCheckpoint{NextProbeHeight: 0}); err != nil {
		t.Fatal(err)
	}
	var s progressiveRawState
	s.InitFromCheckpoint(dir, 10, -1)
	if s.nextProbe != 0 {
		t.Fatalf("nextProbe %d want 0 (resume genesis fetch)", s.nextProbe)
	}
	_ = j
	_ = rs
}
