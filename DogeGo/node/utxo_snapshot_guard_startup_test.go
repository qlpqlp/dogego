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

func TestLoadUtxoSnapshotAtStartupKeepsReplaySnapshot(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 5200)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	utxo := store.NewUtxoCache()
	for h := int64(0); h <= 128; h++ {
		h80, _ := j.ReadHeaderAt(h)
		raw := store.MakeTestBlockRaw(t, h80)
		_ = rs.Put(pow.BlockHashLE(h80), raw)
	}
	utxo.SetTipHeightForTest(5015)
	// Simulate real chainstate substance (fabricated tip-only saves have 0 coins).
	utxo.AddUtxoForTest([36]byte{1}, store.UtxoEntry{Value: 1, Height: 1, PkScript: []byte{0x76, 0xa9}})
	utxo.AddUtxoForTest([36]byte{2}, store.UtxoEntry{Value: 1, Height: 2, PkScript: []byte{0x76, 0xa9}})
	utxo.AddUtxoForTest([36]byte{3}, store.UtxoEntry{Value: 1, Height: 3, PkScript: []byte{0x76, 0xa9}})
	utxo.AddUtxoForTest([36]byte{4}, store.UtxoEntry{Value: 1, Height: 4, PkScript: []byte{0x76, 0xa9}})
	utxo.AddUtxoForTest([36]byte{5}, store.UtxoEntry{Value: 1, Height: 5, PkScript: []byte{0x76, 0xa9}})
	utxo.AddUtxoForTest([36]byte{6}, store.UtxoEntry{Value: 1, Height: 6, PkScript: []byte{0x76, 0xa9}})
	utxo.AddUtxoForTest([36]byte{7}, store.UtxoEntry{Value: 1, Height: 7, PkScript: []byte{0x76, 0xa9}})
	utxo.AddUtxoForTest([36]byte{8}, store.UtxoEntry{Value: 1, Height: 8, PkScript: []byte{0x76, 0xa9}})
	snapPath := store.UtxoSnapshotPath(dir)
	if err := utxo.SaveSnapshot(snapPath); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRawBlockSyncCheckpoint(dir, store.RawBlockSyncCheckpoint{
		NextProbeHeight:     129,
		ContiguousRawHeight: 128,
	}); err != nil {
		t.Fatal(err)
	}
	params, _ := chain.ParamsFor(chain.RebootTestnet)
	got, quarantined, err := LoadUtxoSnapshotAtStartup(snapPath, dir, j, rs, params.Net)
	if err != nil {
		t.Fatal(err)
	}
	if quarantined {
		t.Fatal("replay snapshot should not be quarantined")
	}
	if got.TipHeight() != 5015 {
		t.Fatalf("tip=%d want 5015", got.TipHeight())
	}
}

func TestLoadUtxoSnapshotAtStartupRestoresStale(t *testing.T) {
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
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	good := store.NewUtxoCache()
	good.SetTipHeightForTest(4000)
	for i := 0; i < 10; i++ {
		var k [36]byte
		k[0] = byte(i + 1)
		good.AddUtxoForTest(k, store.UtxoEntry{Value: 1, Height: 1, PkScript: []byte{0x76, 0xa9}})
	}
	stalePath := filepath.Join(dir, "utxo.cache.stale.bodies_missing")
	if err := good.SaveSnapshot(stalePath); err != nil {
		t.Fatal(err)
	}
	bad := store.NewUtxoCache()
	bad.SetTipHeightForTest(12)
	snapPath := store.UtxoSnapshotPath(dir)
	if err := bad.SaveSnapshot(snapPath); err != nil {
		t.Fatal(err)
	}
	params, _ := chain.ParamsFor(chain.RebootTestnet)
	got, quarantined, err := LoadUtxoSnapshotAtStartup(snapPath, dir, j, rs, params.Net)
	if err != nil {
		t.Fatal(err)
	}
	if quarantined {
		t.Fatal("unexpected quarantine")
	}
	if got.TipHeight() != 4000 {
		t.Fatalf("tip=%d want 4000 restored from stale", got.TipHeight())
	}
}
