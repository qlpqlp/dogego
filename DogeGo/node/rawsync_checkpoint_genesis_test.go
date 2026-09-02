// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"path/filepath"
	"testing"
	"time"

	"dogego/pow"
	"dogego/store"
)

func TestSyncCheckpointToContiguousGenesisMissing(t *testing.T) {
	dir := t.TempDir()
	var s progressiveRawState
	s.chainDir = dir
	s.SyncCheckpointToContiguous(-1)
	if s.nextProbe != 0 {
		t.Fatalf("nextProbe %d want 0 when contiguous unknown / genesis missing", s.nextProbe)
	}
}

func TestSyncCheckpointToContiguousAfterGenesis(t *testing.T) {
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
	appendFakeHeaderChain(t, j, genesisRaw[:80], 5)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw); err != nil {
		t.Fatal(err)
	}
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	bs.noteBlockStoredAt(0)
	var s progressiveRawState
	s.chainDir = dir
	s.SyncCheckpointToContiguous(bs.ContiguousRawHeight())
	if s.nextProbe != 1 {
		t.Fatalf("nextProbe %d want 1 after genesis stored", s.nextProbe)
	}
	// Wait for coalesced async flush so TempDir cleanup does not race leftover .tmp files.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		flushing := s.checkpointFlushing || s.checkpointDirty
		s.mu.Unlock()
		if !flushing {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}
