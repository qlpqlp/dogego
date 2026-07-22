// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestExtendStoredBodiesFrontierIncremental(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(dir+"/headers.bin", genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw); err != nil {
		t.Fatal(err)
	}
	var frontier int64 = -1
	if !extendStoredBodiesFrontier(j, raw, &frontier, 0) {
		t.Fatal("height 0")
	}
	if frontier != 0 {
		t.Fatalf("frontier=%d want 0", frontier)
	}
	if !extendStoredBodiesFrontier(j, raw, &frontier, 0) {
		t.Fatal("idempotent extend")
	}
	if !storedBodiesThrough(j, raw, 1) {
		t.Fatal("storedBodiesThrough(1)")
	}
	_ = chain.MainnetDogecoin
}
