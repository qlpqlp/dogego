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

func TestLowestMissingForIBDPrefersConnectGap(t *testing.T) {
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
	params, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, utxo)
	bs.noteBlockStoredAt(0)
	low, err := LowestMissingForIBD(j, rs, bs.ContiguousRawHeight(), 4, bs)
	if err != nil {
		t.Fatal(err)
	}
	if low != 1 {
		t.Fatalf("low=%d want 1 (connect blocked at height 1)", low)
	}
}

func TestConnectFrontierHeightWhenUtxoAhead(t *testing.T) {
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
	appendFakeHeaderChain(t, j, genesisRaw[:80], 5000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	params, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(4418)
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, utxo)
	bs.noteBlockStoredAt(0)
	if got := ConnectNextHeight(bs); got != 4419 {
		t.Fatalf("ConnectNextHeight=%d want 4419", got)
	}
	if got := ConnectFrontierHeight(bs); got != 1 {
		t.Fatalf("ConnectFrontierHeight=%d want 1 (contiguous 0)", got)
	}
	if got := ConnectBodyGapHeight(bs); got != 1 {
		t.Fatalf("ConnectBodyGapHeight=%d want 1", got)
	}
}

func TestPurgeUnreadableBodyAtHeightPreservesHigher(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 2)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw); err != nil {
		t.Fatal(err)
	}
	params, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, store.NewUtxoCache())
	h1, _ := j.ReadHeaderAt(1)
	body1 := make([]byte, 200)
	copy(body1[:80], h1)
	hash1 := pow.BlockHashLE(h1)
	if err := os.WriteFile(filepath.Join(rs.Dir(), hex.EncodeToString(hash1[:])+".bin"), body1, 0o600); err != nil {
		t.Fatal(err)
	}
	bs.SeedContiguousTip(1)
	if cont := bs.ContiguousRawHeight(); cont != 1 {
		t.Fatalf("contiguous=%d want 1", cont)
	}
	h2, _ := j.ReadHeaderAt(2)
	stub := make([]byte, 130)
	copy(stub, h2)
	hash2 := pow.BlockHashLE(h2)
	if err := os.WriteFile(filepath.Join(rs.Dir(), hex.EncodeToString(hash2[:])+".bin"), stub, 0o600); err != nil {
		t.Fatal(err)
	}
	if cont := bs.ContiguousRawHeight(); cont != 1 {
		t.Fatalf("contiguous=%d want 1 with unreadable stub at height 2", cont)
	}
	removed, err := bs.purgeUnreadableBodyAtHeight(2)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("want stub at height 2 removed")
	}
	if cont := bs.ContiguousRawHeight(); cont != 1 {
		t.Fatalf("contiguous=%d want 1 unchanged after height-2 purge only", cont)
	}
	if !store.HasStoredBodyAtHeight(j, rs, 1, params.Net) {
		t.Fatal("height 1 body should remain")
	}
}

func TestConnectBodyGapHeight(t *testing.T) {
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
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	params, _ := chain.ParamsFor(chain.RebootTestnet)
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, store.NewUtxoCache())
	if got := ConnectBodyGapHeight(bs); got != 0 {
		t.Fatalf("gap=%d want 0 without genesis body", got)
	}
}
