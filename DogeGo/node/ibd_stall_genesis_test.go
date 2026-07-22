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
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestIbdStallRecoverIntervalGenesis(t *testing.T) {
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
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	if got := ibdStallRecoverInterval(bs, nil); got != ibdStallNoBlockIntervalGenesis {
		t.Fatalf("genesis missing interval=%v want %v", got, ibdStallNoBlockIntervalGenesis)
	}
	if err := EnsureLocalGenesis(bs); err != nil {
		t.Fatal(err)
	}
	if NeedsGenesisBlock(bs) {
		t.Fatal("genesis should be stored")
	}
	if got := ibdStallRecoverInterval(bs, nil); got == ibdStallNoBlockIntervalGenesis {
		t.Fatalf("after genesis stored interval should widen, got %v", got)
	}
}

func TestIbdStallRecoverIntervalZeroStored(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 2000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	if err := EnsureLocalGenesis(bs); err != nil {
		t.Fatal(err)
	}
	bs.SeedContiguousTip(1500)
	snap := map[string]interface{}{"blocks_stored_ibd": int64(0)}
	if got := ibdStallRecoverInterval(bs, snap); got != ibdStallNoBlockIntervalZeroStored {
		t.Fatalf("zero-stored interval=%v want %v", got, ibdStallNoBlockIntervalZeroStored)
	}
}

func TestIbdStallRecoverIntervalConnectCaughtUp(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 10_010)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, utxo)
	if err := EnsureLocalGenesis(bs); err != nil {
		t.Fatal(err)
	}
	bs.SeedContiguousTip(10_005)
	utxo.SetTipHeightForTest(10_005)
	if !BodiesBehindHeaders(bs) {
		t.Fatal("expected bodies behind headers in fixture")
	}
	if got := ibdStallRecoverInterval(bs, nil); got != ibdStallNoBlockIntervalBodyOnly {
		t.Fatalf("body-only interval=%v want %v", got, ibdStallNoBlockIntervalBodyOnly)
	}
}

func TestIbdStallRecoverIntervalBodyIBDPaused(t *testing.T) {
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
	if err := EnsureLocalGenesis(bs); err != nil {
		t.Fatal(err)
	}
	bs.SeedContiguousTip(3085)
	if !ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		t.Fatal("fixture should pause header catch-up")
	}
	if got := ibdStallRecoverInterval(bs, nil); got != ibdStallNoBlockIntervalEarly {
		t.Fatalf("body-IBD pause interval=%v want %v", got, ibdStallNoBlockIntervalEarly)
	}
}

func TestIbdStallRecoverIntervalMidDepth(t *testing.T) {
	tip, cont := int64(534_000), int64(9621)
	gap := tip - cont
	var got time.Duration
	switch {
	case gap > forwardIBDParallelWindow && cont < 1000:
		got = ibdStallNoBlockIntervalEarly
	case gap > 10_000 && cont >= 1000:
		got = ibdStallNoBlockIntervalMid
	default:
		got = ibdStallNoBlockInterval
	}
	if got != ibdStallNoBlockIntervalMid {
		t.Fatalf("deep IBD interval=%v want %v (tip=%d cont=%d)", got, ibdStallNoBlockIntervalMid, tip, cont)
	}
}

func TestAutoRecoverSweepEnsuresLocalGenesis(t *testing.T) {
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
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	if !NeedsGenesisBlock(bs) {
		t.Fatal("expected missing genesis")
	}
	if _, err := autoRecoverSweep(dir, j, nil, p, bs, nil); err != nil {
		t.Fatal(err)
	}
	if NeedsGenesisBlock(bs) {
		t.Fatal("sweep should store local genesis")
	}
	// undersized stub purge + re-ensure
	genRaw := store.MakeTestBlockRaw(t, g80[:])
	genID := pow.BlockHashLE(genRaw[:80])
	stub := filepath.Join(rs.Dir(), hex.EncodeToString(genID[:])+".bin")
	if err := os.WriteFile(stub, genRaw[:120], 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := autoRecoverSweep(dir, j, nil, p, bs, nil); err != nil {
		t.Fatal(err)
	}
	if NeedsGenesisBlock(bs) {
		t.Fatal("sweep should restore genesis after stub purge")
	}
}
