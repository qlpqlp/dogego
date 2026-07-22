// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/pow"
	"dogego/store"
)

func TestHeaderRewindHeightBeforeRetarget_postDigiShield(t *testing.T) {
	got := headerRewindHeightBeforeRetarget(510_456, 1)
	want := int64(510_456 - legacyDifficultyPeriodBlocks)
	if got != want {
		t.Fatalf("got %d want %d", got, want)
	}
	if got := headerRewindHeightBeforeRetarget(100, 1); got != 0 {
		t.Fatalf("near genesis got %d want 0", got)
	}
	if got := headerRewindHeightBeforeRetarget(10_000, 2016); got != 8064 {
		t.Fatalf("legacy interval: got %d want 8064", got)
	}
}

func TestShouldDeferHeaderTipTruncateDuringBodyIBD(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(dir+"/h.bin", genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, genesisRaw[:80], 510_000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	bs.contiguousTip = 8_592
	if !BodiesBehindHeaders(bs) {
		t.Fatal("want bodies behind headers")
	}
	if !shouldDeferHeaderTipTruncateDuringBodyIBD(bs, 510_454, 508_000) {
		t.Fatal("large header/body gap should defer truncate")
	}
	bs.contiguousTip = 510_000
	if shouldDeferHeaderTipTruncateDuringBodyIBD(bs, 510_454, 508_000) {
		t.Fatal("bodies near header tip should allow truncate")
	}
}
