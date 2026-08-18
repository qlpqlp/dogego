// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestUtxoStartupConnectNeeded(t *testing.T) {
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(100)
	bs := &BlockStoreCtx{}
	bs.SeedContiguousTip(100)
	if utxoStartupConnectNeeded(bs, utxo) {
		t.Fatal("aligned: no startup connect")
	}
	bs.SeedContiguousTip(105)
	if !utxoStartupConnectNeeded(bs, utxo) {
		t.Fatal("lag: startup connect needed")
	}
}

func TestUtxoStartupConnectNeededDefersDuringDownloadFirst(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderChain(dir, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, g80[:], 534_000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(6_702)
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, utxo)
	if err := EnsureLocalGenesis(bs); err != nil {
		t.Fatal(err)
	}
	bs.SeedContiguousTip(54_506)
	if utxoStartupConnectNeeded(bs, utxo) {
		t.Fatal("download-first IBD must skip startup connect while bodies lag headers")
	}
}

func TestBodiesAlignedForUtxoSnapshot(t *testing.T) {
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(1000)
	bs := &BlockStoreCtx{}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 1000
	bs.contiguousMu.Unlock()
	if !BodiesAlignedForUtxoSnapshot(bs, utxo) {
		t.Fatal("aligned at same height")
	}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 900
	bs.contiguousMu.Unlock()
	if BodiesAlignedForUtxoSnapshot(bs, utxo) {
		t.Fatal("not aligned when bodies far behind")
	}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 980
	bs.contiguousMu.Unlock()
	if !BodiesAlignedForUtxoSnapshot(bs, utxo) {
		t.Fatal("aligned within margin")
	}
}

func TestShouldPersistSyncCheckpointDuringReplay(t *testing.T) {
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(5000)
	bs := &BlockStoreCtx{Utxo: utxo}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 200
	bs.contiguousMu.Unlock()
	if !shouldPersistSyncCheckpoint(256, bs) {
		t.Fatal("want checkpoint every 16 during replay")
	}
	if shouldPersistSyncCheckpoint(255, bs) {
		t.Fatal("255 not on 16 boundary")
	}
	if !shouldPersistSyncCheckpoint(64, nil) {
		t.Fatal("want 64 boundary when not replaying")
	}
}

func TestShouldPersistSyncCheckpointDuringDeepBodyIBD(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderChain(dir, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, g80[:], 534_000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	bs.SeedContiguousTip(5896)
	if !shouldPersistSyncCheckpoint(5904, bs) {
		t.Fatal("want checkpoint every 16 during early deep body IBD")
	}
	if shouldPersistSyncCheckpoint(5905, bs) {
		t.Fatal("5905 not on 16 boundary")
	}
}

func TestShouldSkipUtxoSnapshotDowngrade(t *testing.T) {
	dir := t.TempDir()
	path := store.UtxoSnapshotPath(dir)
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(17_056)
	if err := utxo.SaveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	if !shouldSkipUtxoSnapshotDowngrade(path, 2_058) {
		t.Fatal("2058 should skip downgrade vs 17056")
	}
	if !shouldSkipUtxoSnapshotDowngrade(path, 2_000) {
		t.Fatal("2000 should skip downgrade vs 17056")
	}
	if shouldSkipUtxoSnapshotDowngrade(path, 16_900) {
		t.Fatal("16900 within margin of 17056 should not skip")
	}
}

func TestPersistUtxoSnapshotIfAlignedSkipsDowngrade(t *testing.T) {
	dir := t.TempDir()
	path := store.UtxoSnapshotPath(dir)
	high := store.NewUtxoCache()
	high.SetTipHeightForTest(10_000)
	if err := high.SaveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	bs := &BlockStoreCtx{}
	bs.SeedContiguousTip(10_000)
	low := store.NewUtxoCache()
	low.SetTipHeightForTest(500)
	if err := PersistUtxoSnapshotIfAligned(bs, low, path, "test"); err != nil {
		t.Fatal(err)
	}
	diskTip, err := store.ReadUtxoSnapshotDiskTip(path)
	if err != nil {
		t.Fatal(err)
	}
	if diskTip != 10_000 {
		t.Fatalf("downgrade blocked: disk tip %d want 10000", diskTip)
	}
}
