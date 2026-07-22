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
	"dogego/consensus"
	"dogego/pow"
	"dogego/store"
)

func TestShouldUpdateFeeHistoryOnConnectDuringIBD(t *testing.T) {
	bs := &BlockStoreCtx{
		FeeHistory:          consensus.NewFeeHistory(0),
		FeeHistoryPath:      "fee.json",
		FeeEstimatesDatPath: "fee.dat",
		contiguousTip:       5000,
	}
	if !bs.shouldUpdateFeeHistoryOnConnect() {
		t.Fatal("want fee history updates when not in body IBD")
	}
}

func TestHasAncestorBlockBodies(t *testing.T) {
	dir := t.TempDir()
	blockRaw, _ := store.TestMinimalBlock()
	j, err := store.OpenHeaderJournal(dir+"/headers.bin", blockRaw[:80])
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
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, nil)
	if !bs.hasAncestorBlockBodies(0) {
		t.Fatal("height 0")
	}
	if bs.hasAncestorBlockBodies(1) {
		t.Fatal("genesis body missing")
	}
	if err := rs.Put(pow.BlockHashLE(blockRaw[:80]), blockRaw); err != nil {
		t.Fatal(err)
	}
	if !bs.hasAncestorBlockBodies(1) {
		t.Fatal("want genesis for height 1")
	}
	bs.noteBlockStoredAt(0)
	if bs.contiguousTip != 0 {
		t.Fatalf("contiguous tip %d", bs.contiguousTip)
	}
	if !bs.hasAncestorBlockBodies(1) {
		t.Fatal("fast path: contiguous tip covers height 1")
	}
}

func TestDeferConnectDuringIBDOrphanAhead(t *testing.T) {
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
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, nil)
	bs.SeedContiguousTip(1)
	if bs.forwardIBDGap() <= forwardIBDParallelWindow {
		t.Fatalf("gap %d", bs.forwardIBDGap())
	}
	if !bs.deferConnectDuringIBD(5000) {
		t.Fatal("orphan far ahead should defer connect above forward-IBD window")
	}
	if bs.deferConnectDuringIBD(50) {
		t.Fatal("near frontier should connect")
	}
}

func TestShrinkContiguousTipAfterBodyRemovedDuringReplay(t *testing.T) {
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
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
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
		bs.noteBlockStoredAt(h)
	}
	if cont := bs.ContiguousRawHeight(); cont != 5 {
		t.Fatalf("contiguous=%d want 5", cont)
	}
	bs.shrinkContiguousTipAfterBodyRemoved(3)
	if cont := bs.ContiguousRawHeight(); cont != 5 {
		t.Fatalf("ancient shrink during replay should preserve contiguous=%d want 5", cont)
	}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 6
	bs.contiguousMu.Unlock()
	bs.shrinkContiguousTipAfterBodyRemoved(6)
	if cont := bs.ContiguousRawHeight(); cont != 5 {
		t.Fatalf("frontier shrink during replay contiguous=%d want 5", cont)
	}
}

func TestShrinkAncientHeightDuringReplayPreservesTip(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 100)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	params, _ := chain.ParamsFor(chain.RebootTestnet)
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(200)
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, utxo)

	bs.contiguousMu.Lock()
	bs.contiguousTip = 100
	bs.contiguousMu.Unlock()

	bs.shrinkContiguousTipAfterBodyRemoved(26)
	if cont := bs.ContiguousRawHeight(); cont != 100 {
		t.Fatalf("ancient purge during replay must not shrink contiguous=%d want 100", cont)
	}
	bs.shrinkContiguousTipAfterBodyRemoved(101)
	if cont := bs.ContiguousRawHeight(); cont != 100 {
		t.Fatalf("frontier purge during replay contiguous=%d want 100", cont)
	}
}

func TestSkipConnectDuringUtxoSnapshotReplay(t *testing.T) {
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
	appendFakeHeaderChain(t, j, genesisRaw[:80], 100)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	params, _ := chain.ParamsFor(chain.RebootTestnet)
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(50)
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, utxo)
	bs.noteBlockStoredAt(0)
	if !bs.utxoAheadOfStoredBodies() {
		t.Fatal("want utxo ahead during replay")
	}
	bs.tryConnectContiguousFrontier()
	if bs.ContiguousRawHeight() != 0 {
		t.Fatalf("connect should not run during snapshot replay; contiguous=%d", bs.ContiguousRawHeight())
	}
	bs.FlushDeferredConnect()
	if bs.ContiguousRawHeight() != 0 {
		t.Fatalf("FlushDeferredConnect should skip during snapshot replay")
	}
}

func TestRefreshContiguousTipExtendsFromDisk(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 1)
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
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, nil)
	bs.noteBlockStoredAt(0)
	h1, err := j.ReadHeaderAt(1)
	if err != nil {
		t.Fatal(err)
	}
	body1 := make([]byte, 200)
	copy(body1[:80], h1)
	hash1 := pow.BlockHashLE(h1)
	name := hex.EncodeToString(hash1[:]) + ".bin"
	if err := os.WriteFile(filepath.Join(dir, "rawblocks", name), body1, 0o600); err != nil {
		t.Fatal(err)
	}
	bs.SeedContiguousTip(0)
	if got := bs.RefreshContiguousTip(); got != 1 {
		t.Fatalf("refresh contiguous=%d want 1", got)
	}
}

func TestRefreshContiguousTipNotifiesAdvance(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 1)
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
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, nil)
	bs.noteBlockStoredAt(0)
	h1, err := j.ReadHeaderAt(1)
	if err != nil {
		t.Fatal(err)
	}
	body1 := make([]byte, 200)
	copy(body1[:80], h1)
	hash1 := pow.BlockHashLE(h1)
	name := hex.EncodeToString(hash1[:]) + ".bin"
	if err := os.WriteFile(filepath.Join(dir, "rawblocks", name), body1, 0o600); err != nil {
		t.Fatal(err)
	}
	bs.SeedContiguousTip(0)
	var notified int64 = -1
	bs.SetOnContiguousAdvance(func(cont int64) { notified = cont })
	if got := bs.RefreshContiguousTip(); got != 1 {
		t.Fatalf("refresh contiguous=%d want 1", got)
	}
	if notified != 1 {
		t.Fatalf("onContiguousAdvance notified=%d want 1", notified)
	}
}

func TestDeferConnectNeverBelowForwardIBDWindow(t *testing.T) {
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
	params, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, nil)
	bs.SeedContiguousTip(2)
	av := consensus.NewAssumeValid("mainnet", consensus.DefaultAssumeValidHex("mainnet"))
	av.PinResolvedHeight(5_050_000)
	av.SetHeaderTip(4999)
	bs.AssumeValid = av
	if bs.deferConnectDuringIBD(2) {
		t.Fatal("height 2 must connect immediately (coinbase subsidy) even during assume-valid IBD")
	}
	if !bs.deferConnectDuringIBD(5000) {
		t.Fatal("deep assume-valid bulk IBD should defer connect at height 5000")
	}
}

func TestNoteBlockStoredAtSequentialRequiresAdequateBody(t *testing.T) {
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
	appendFakeHeaderChain(t, j, genesisRaw[:80], 2)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	params, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, nil)
	bs.noteBlockStoredAt(0)
	if bs.contiguousTip >= 0 {
		t.Fatalf("contiguous=%d want -1 without stored genesis body", bs.contiguousTip)
	}
	if err := rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw); err != nil {
		t.Fatal(err)
	}
	bs.noteBlockStoredAt(0)
	if bs.contiguousTip != 0 {
		t.Fatalf("contiguous=%d want 0 after genesis stored", bs.contiguousTip)
	}
	bs.noteBlockStoredAt(1)
	if bs.contiguousTip != 0 {
		t.Fatalf("contiguous=%d want 0 when height 1 body missing", bs.contiguousTip)
	}
}
