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

func TestPrepareAtStartupArmsSyncAtGenesisTip(t *testing.T) {
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
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	var s progressiveRawState
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	s.PrepareAtStartup(bs)
	if s.idleFull {
		t.Fatal("expected block sync armed at header tip 0 (genesis body missing)")
	}
	if s.nextProbe != 0 {
		t.Fatalf("nextProbe %d want 0", s.nextProbe)
	}
	b, ok := s.claimBatch(bs, 0)
	if !ok || len(b.heights) == 0 || b.heights[0] != 0 {
		t.Fatalf("claim at tip 0 ok=%v heights=%v", ok, b.heights)
	}
}
