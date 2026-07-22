// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestAdvanceReplayContiguousFromParallelBodies(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 10)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw); err != nil {
		t.Fatal(err)
	}
	params, _ := chain.ParamsFor(chain.RebootTestnet)
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(50)
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, utxo)
	bs.noteBlockStoredAt(0)
	for h := int64(1); h <= 5; h++ {
		hdr, _ := j.ReadHeaderAt(h)
		body := make([]byte, 200)
		copy(body[:80], hdr)
		hash := pow.BlockHashLE(hdr)
		if err := os.WriteFile(filepath.Join(rs.Dir(), hex.EncodeToString(hash[:])+".bin"), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := bs.ContiguousRawHeight(); got != 0 {
		t.Fatalf("contiguous=%d want 0 before replay advance", got)
	}
	if got := bs.AdvanceReplayContiguousFromDisk(512); got != 5 {
		t.Fatalf("after replay advance contiguous=%d want 5", got)
	}
}

func TestAdvanceReplayContiguousStopsAtGap(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 5)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw); err != nil {
		t.Fatal(err)
	}
	// Body at height 2 only (gap at 1).
	hdr2, _ := j.ReadHeaderAt(2)
	body2 := make([]byte, 200)
	copy(body2[:80], hdr2)
	hash2 := pow.BlockHashLE(hdr2)
	if err := os.WriteFile(filepath.Join(rs.Dir(), hex.EncodeToString(hash2[:])+".bin"), body2, 0o600); err != nil {
		t.Fatal(err)
	}
	params, _ := chain.ParamsFor(chain.RebootTestnet)
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(50)
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, utxo)
	bs.noteBlockStoredAt(0)
	if got := bs.AdvanceReplayContiguousFromDisk(512); got != 0 {
		t.Fatalf("contiguous=%d want 0 (gap at height 1)", got)
	}
}

func TestRampReplayContiguousFromDisk(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 20)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw); err != nil {
		t.Fatal(err)
	}
	for h := int64(1); h <= 15; h++ {
		hdr, _ := j.ReadHeaderAt(h)
		body := make([]byte, 200)
		copy(body[:80], hdr)
		hash := pow.BlockHashLE(hdr)
		if err := os.WriteFile(filepath.Join(rs.Dir(), hex.EncodeToString(hash[:])+".bin"), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	params, _ := chain.ParamsFor(chain.RebootTestnet)
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(50)
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, utxo)
	bs.noteBlockStoredAt(0)
	if got := bs.RampReplayContiguousFromDisk(); got != 15 {
		t.Fatalf("ramp contiguous=%d want 15", got)
	}
}
