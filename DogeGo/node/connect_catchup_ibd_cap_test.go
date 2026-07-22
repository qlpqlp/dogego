// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/chain"
	"dogego/consensus"
	"dogego/pow"
	"dogego/store"
)

func TestConnectCatchUpBlocksPerIBDCall(t *testing.T) {
	bs := &BlockStoreCtx{contiguousTip: 10000, Utxo: store.NewUtxoCache()}
	bs.Utxo.SetTipHeightForTest(1000)
	if got := connectCatchUpBlocksPerIBDCall(bs); got != 16 {
		t.Fatalf("large lag blocks=%d want 16", got)
	}
	bs.Utxo.SetTipHeightForTest(8500)
	if got := connectCatchUpBlocksPerIBDCall(bs); got != 4 {
		t.Fatalf("small lag blocks=%d want 4", got)
	}
}

func TestConnectCatchUpBlocksPerIBDCallBodyPause(t *testing.T) {
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
	bs.SeedContiguousTip(616)
	bs.Utxo = store.NewUtxoCache()
	bs.Utxo.SetTipHeightForTest(600)
	if got := connectCatchUpBlocksPerIBDCall(bs); got != 16 {
		t.Fatalf("paused body IBD blocks=%d want 16", got)
	}
}

func TestConnectCatchUpPassesBoostDuringBodyIBDPause(t *testing.T) {
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
	bs.SeedContiguousTip(616)
	if got := connectCatchUpPasses(100, bs); got != 2 {
		t.Fatalf("paused body IBD passes=%d want 2", got)
	}
	bs.SeedContiguousTip(7700)
	bs.Utxo = store.NewUtxoCache()
	bs.Utxo.SetTipHeightForTest(6397)
	if got := connectCatchUpPasses(1302, bs); got != 4 {
		t.Fatalf("large lag passes=%d want 4", got)
	}
	if got := connectCatchUpPasses(9000, bs); got != 8 {
		t.Fatalf("very large lag passes=%d want 8", got)
	}
	if got := connectCatchUpBlocksPerIBDCall(bs); got < 16 {
		t.Fatalf("large lag blocks=%d want >=16", got)
	}
	bs.Utxo.SetTipHeightForTest(1000)
	bs.SeedContiguousTip(20_000)
	if got := connectCatchUpBlocksPerIBDCall(bs); got != 128 {
		t.Fatalf("post-restart lag blocks=%d want 128", got)
	}
}

func TestConnectCatchUpAssumeValidBatchBoost(t *testing.T) {
	av := consensus.NewAssumeValid("mainnet", consensus.DefaultAssumeValidHex("mainnet"))
	av.PinResolvedHeight(5_050_000)
	av.SetHeaderTip(534_000)
	consensus.SetGlobalAssumeValid(av)
	defer consensus.SetGlobalAssumeValid(nil)

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
	bs.Utxo = store.NewUtxoCache()
	bs.SeedContiguousTip(20_000)
	bs.Utxo.SetTipHeightForTest(1000)
	if got := connectCatchUpBlocksPerIBDCall(bs); got != 512 {
		t.Fatalf("assume-valid skip blocks=%d want 512", got)
	}
	if got := connectCatchUpPasses(19_000, bs); got != 16 {
		t.Fatalf("assume-valid skip passes=%d want 16", got)
	}
}

func TestConnectUtxoLockChunkSize(t *testing.T) {
	if got := connectUtxoLockChunkSize(512, nil); got != 64 {
		t.Fatalf("default chunk=%d want 64", got)
	}
	if got := connectUtxoLockChunkSize(32, nil); got != 32 {
		t.Fatalf("small chunk=%d want 32", got)
	}
	av := consensus.NewAssumeValid("mainnet", consensus.DefaultAssumeValidHex("mainnet"))
	av.PinResolvedHeight(5_050_000)
	av.SetHeaderTip(534_000)
	consensus.SetGlobalAssumeValid(av)
	defer consensus.SetGlobalAssumeValid(nil)
	bs := &BlockStoreCtx{Utxo: store.NewUtxoCache()}
	bs.Utxo.SetTipHeightForTest(10_000)
	if got := connectUtxoLockChunkSize(512, bs); got != 128 {
		t.Fatalf("assume-valid chunk=%d want 128", got)
	}
}
