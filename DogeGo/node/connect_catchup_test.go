// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"
	"time"

	"dogego/chain"
	"dogego/consensus"
	"dogego/pow"
	"dogego/store"
)

func TestConnectCatchUpLag(t *testing.T) {
	bs := &BlockStoreCtx{contiguousTip: 8600}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(2200)
	if lag := ConnectCatchUpLag(bs, utxo); lag != 6400 {
		t.Fatalf("lag=%d want 6400", lag)
	}
	utxo.SetTipHeightForTest(8600)
	if lag := ConnectCatchUpLag(bs, utxo); lag != 0 {
		t.Fatalf("caught up lag=%d want 0", lag)
	}
}

func TestConnectFrontierMaxStepsScalesWithLag(t *testing.T) {
	bs := &BlockStoreCtx{
		contiguousTip: 9000,
		Utxo:          store.NewUtxoCache(),
		Journal:       nil,
	}
	bs.Utxo.SetTipHeightForTest(500)
	if got := connectFrontierMaxSteps(bs); got != 4096 {
		t.Fatalf("large lag maxSteps=%d want 4096", got)
	}
	bs.Utxo.SetTipHeightForTest(8500)
	if got := connectFrontierMaxSteps(bs); got != 512 {
		t.Fatalf("small lag maxSteps=%d want 512", got)
	}
}

func TestSyncUtxoMaxConnectPassesWithoutJournal(t *testing.T) {
	bs := &BlockStoreCtx{
		contiguousTip: 9500,
		Utxo:          store.NewUtxoCache(),
	}
	bs.Utxo.SetTipHeightForTest(1000)
	if got := syncUtxoMaxConnectPasses(bs, 9500); got != 128 {
		t.Fatalf("without journal passes=%d want 128", got)
	}
}

func TestIBDConnectBlocksPerMinute(t *testing.T) {
	ibdConnectRate.mu.Lock()
	ibdConnectRate.samples = nil
	ibdConnectRate.mu.Unlock()
	now := time.Now()
	RecordIBDConnectAdvance(100)
	RecordIBDConnectAdvance(400)
	ibdConnectRate.mu.Lock()
	if len(ibdConnectRate.samples) != 2 {
		t.Fatalf("samples=%d want 2", len(ibdConnectRate.samples))
	}
	ibdConnectRate.samples[0].at = now.Add(-2 * time.Minute)
	ibdConnectRate.samples[1].at = now
	ibdConnectRate.mu.Unlock()
	rate := IBDConnectBlocksPerMinute()
	if rate < 140 || rate > 160 {
		t.Fatalf("rate=%f want ~150/min", rate)
	}
}

func TestConnectCatchUpPasses(t *testing.T) {
	if got := connectCatchUpPasses(100, nil); got != 1 {
		t.Fatalf("passes=%d want 1", got)
	}
	if got := connectCatchUpPasses(3000, nil); got != 2 {
		t.Fatalf("passes=%d want 2", got)
	}
	if got := connectCatchUpPasses(5000, nil); got != 3 {
		t.Fatalf("passes=%d want 3", got)
	}
	if got := connectCatchUpPasses(10_000, nil); got != 4 {
		t.Fatalf("passes=%d want 4", got)
	}
}

func testBodyIBDBlockStoreCtx(t *testing.T, headerTip, contiguousTip, utxoTip int64) *BlockStoreCtx {
	t.Helper()
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
	appendFakeHeaderChain(t, j, g80[:], int(headerTip))
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	if err := EnsureLocalGenesis(bs); err != nil {
		t.Fatal(err)
	}
	bs.SeedContiguousTip(contiguousTip)
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(utxoTip)
	bs.Utxo = utxo
	return bs
}

func TestConnectCatchUpPassesDeepBodyIBD(t *testing.T) {
	bs := testBodyIBDBlockStoreCtx(t, 534_000, 20_000, 5000)
	if !ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		t.Fatal("expected body IBD pause")
	}
	if got := connectCatchUpPasses(10_000, bs); got != 8 {
		t.Fatalf("passes=%d want 8 during body IBD lag>8192", got)
	}
	if got := connectCatchUpPasses(600, bs); got != 4 {
		t.Fatalf("passes=%d want 4 during body IBD lag>512", got)
	}
}

func TestConnectCatchUpBlocksPerIBDCallDeepBody(t *testing.T) {
	bs := testBodyIBDBlockStoreCtx(t, 534_000, 20_000, 5000)
	if got := connectCatchUpBlocksPerIBDCall(bs); got != 128 {
		t.Fatalf("blocksPerCall=%d want 128 for lag>8192 with scripts", got)
	}
	bs.Utxo.SetTipHeightForTest(17_500)
	if got := connectCatchUpBlocksPerIBDCall(bs); got != 64 {
		t.Fatalf("blocksPerCall=%d want 64 for lag>2048", got)
	}
}

func TestSyncUtxoMaxConnectPassesDeepBacklog(t *testing.T) {
	bs := testBodyIBDBlockStoreCtx(t, 534_000, 20_000, 1000)
	if got := syncUtxoMaxConnectPasses(bs, 20_000); got != 512 {
		t.Fatalf("passes=%d want 512 for backlog>8192", got)
	}
	if got := syncUtxoMaxConnectPasses(bs, 2500); got != 256 {
		t.Fatalf("passes=%d want 256 for backlog>512", got)
	}
}

func TestShouldUpdateFeeHistoryOnConnect(t *testing.T) {
	bs := &BlockStoreCtx{FeeHistory: consensus.NewFeeHistory(0)}
	if !bs.shouldUpdateFeeHistoryOnConnect() {
		t.Fatal("without journal: fee history updates allowed when not in body IBD")
	}
}

func TestConnectCatchUpInterval(t *testing.T) {
	if d := connectCatchUpInterval(10_000); d != 500*time.Millisecond {
		t.Fatalf("10k lag interval=%v want 500ms", d)
	}
	if d := connectCatchUpInterval(3000); d != time.Second {
		t.Fatalf("3k lag interval=%v want 1s", d)
	}
	if d := connectCatchUpInterval(100); d != connectCatchUpPollInterval {
		t.Fatalf("small lag interval=%v want default", d)
	}
}

func TestEffectiveConnectCatchUpMinLag(t *testing.T) {
	if got := EffectiveConnectCatchUpMinLag(nil); got != connectCatchUpMinLag {
		t.Fatalf("nil bs lag=%d want %d", got, connectCatchUpMinLag)
	}
	bs := &BlockStoreCtx{contiguousTip: 744, Journal: nil}
	if got := EffectiveConnectCatchUpMinLag(bs); got != connectCatchUpMinLag {
		t.Fatalf("no journal lag=%d want default", got)
	}
}

func TestPostBatchConnectLagThreshold(t *testing.T) {
	if got := PostBatchConnectLagThreshold(nil); got != 512 {
		t.Fatalf("nil bs threshold=%d want 512", got)
	}
}

func TestShouldPostBatchInlineConnect(t *testing.T) {
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
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(615)
	bs.Utxo = utxo
	if shouldPostBatchInlineConnect(bs) {
		t.Fatal("want false while body missing at connect frontier")
	}
	if !ShouldDeferConnectForBodyDownload(bs) {
		t.Fatal("want download-first defer while headers lead bodies by >1024")
	}
}

func TestCaughtUpConnectMaxBlocks(t *testing.T) {
	if got := caughtUpConnectMaxBlocks(3); got != 3 {
		t.Fatalf("small lag=%d want 3", got)
	}
	if got := caughtUpConnectMaxBlocks(100); got != 32 {
		t.Fatalf("mid lag=%d want 32", got)
	}
	if got := caughtUpConnectMaxBlocks(900); got != 128 {
		t.Fatalf("large lag=%d want 128", got)
	}
}

func TestMaybeSyncConnectCatchUpRespectsInterval(t *testing.T) {
	bs := &BlockStoreCtx{contiguousTip: 5000}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(100)
	last := time.Now()
	MaybeSyncConnectCatchUp(bs, nil, &last)
	before := last
	MaybeSyncConnectCatchUp(bs, utxo, &last)
	if !last.Equal(before) {
		t.Fatal("expected interval gate with nil journal deps")
	}
}
