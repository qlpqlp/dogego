// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestPreferConnectFrontierMissing(t *testing.T) {
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
	for i := 0; i < 5; i++ {
		prev, _ := j.ReadHeaderAt(int64(i))
		h := append([]byte(nil), prev...)
		ph := pow.BlockHashLE(prev)
		copy(h[4:36], ph[:])
		h[76] ^= byte(i + 1)
		if err := j.AppendHeaders([][]byte{h}); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw); err != nil {
		t.Fatal(err)
	}
	net := chain.RebootTestnet
	if got := PreferConnectFrontierMissing(j, rs, 4, 1, net); got != 1 {
		t.Fatalf("low=4 connectNext=1 got %d want 1", got)
	}
	if got := PreferConnectFrontierMissing(j, rs, 1, 1, net); got != 1 {
		t.Fatalf("already missing at connectNext got %d want 1", got)
	}
	if got := PreferConnectFrontierMissing(j, rs, 50_000, 1, net); got != 50_000 {
		t.Fatalf("deep body IBD must not collapse getdata to height 1: got %d", got)
	}
}

func TestRealignProbeToConnectFrontier(t *testing.T) {
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
	params, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, store.NewUtxoCache())
	var rawFill progressiveRawState
	rawFill.chainDir = dir
	rawFill.nextProbe = 500
	rawFill.realignProbeToConnectFrontier(bs, 0)
	if rawFill.nextProbe != 0 {
		t.Fatalf("nextProbe=%d want 0", rawFill.nextProbe)
	}
}
