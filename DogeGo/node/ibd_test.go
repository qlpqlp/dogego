// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/binary"
	"testing"

	"dogego/chain"
	"dogego/consensus"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func TestShouldDeferTxIndexOnPut(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(dir+"/h.bin", genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, genesisRaw[:80], 2000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	bs.noteBlockStoredAt(0)
	if !ShouldDeferTxIndexOnPut(bs) {
		t.Fatal("want defer when headers far ahead of bodies")
	}
	tip, err := j.TipHeight()
	if err != nil {
		t.Fatal(err)
	}
	bs.contiguousMu.Lock()
	bs.contiguousTip = tip - 100 // within tipBackfillDeferGap
	bs.contiguousMu.Unlock()
	if ShouldDeferTxIndexOnPut(bs) {
		t.Fatal("want no defer when bodies nearly catch headers")
	}
}

func TestShouldFillContiguousFrontierFirst(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(dir+"/h.bin", genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, genesisRaw[:80], 2000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	if !shouldFillContiguousFrontierFirst(bs, 1) {
		t.Fatal("want frontier-first when only genesis is contiguous")
	}
	if !shouldFillContiguousFrontierFirst(bs, 0) {
		t.Fatal("want frontier-first at genesis (height 0)")
	}
}

func TestBodiesBehindHeaders_genesisOnly(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(dir+"/h.bin", genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	if !BodiesBehindHeaders(bs) {
		t.Fatal("genesis header without genesis body should report bodies behind")
	}
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	bs.noteBlockStoredAt(0)
	if BodiesBehindHeaders(bs) {
		t.Fatal("genesis body stored at tip 0 should not lag")
	}
}

func TestEffectiveBlockSyncWorkers(t *testing.T) {
	if g := EffectiveBlockSyncWorkers(12, 0); g != 11 {
		t.Fatalf("auto from maxoutbound 12: got %d want 11", g)
	}
	if g := EffectiveBlockSyncWorkers(8, 0); g != 7 {
		t.Fatalf("auto from maxoutbound 8: got %d want 7", g)
	}
	if g := EffectiveBlockSyncWorkers(3, 0); g != 3 {
		t.Fatalf("auto from maxoutbound 3: got %d want 3", g)
	}
	if g := EffectiveBlockSyncWorkers(8, 4); g != 4 {
		t.Fatalf("explicit 4: got %d", g)
	}
	if g := EffectiveBlockSyncWorkers(8, 99); g != maxBlockAssistWorkers {
		t.Fatalf("cap: got %d", g)
	}
	if g := EffectiveBlockSyncWorkersOpt(8, 0, true); g != 8 {
		t.Fatalf("ibd optimize auto from maxoutbound 8: got %d want 8", g)
	}
	if g := EffectiveBlockSyncWorkersOpt(3, 0, true); g != 5 {
		t.Fatalf("ibd optimize floor boost: got %d want 5", g)
	}
	if g := EffectiveBlockSyncWorkersOpt(8, 4, true); g != 4 {
		t.Fatalf("explicit workers ignore optimize boost: got %d", g)
	}
}

func appendFakeHeaderChain(t *testing.T, j *store.HeaderJournal, prev []byte, count int) {
	t.Helper()
	h := append([]byte(nil), prev...)
	headers := make([][]byte, 0, count)
	for i := 0; i < count; i++ {
		h = append([]byte(nil), h...)
		binary.LittleEndian.PutUint64(h[36:44], uint64(i+1))
		cp := make([]byte, 80)
		copy(cp, h)
		headers = append(headers, cp)
	}
	if err := j.AppendHeaders(headers); err != nil {
		t.Fatal(err)
	}
}

func TestShouldDeferInboundHeaders(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(dir+"/h.bin", genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, g80[:], 600)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	bs.noteBlockStoredAt(0)
	if !ShouldDeferInboundHeaders(bs) {
		t.Fatal("bodies at genesis only vs long header chain should defer inbound headers")
	}
	if ShouldAnnounceConnectedBlocks(bs) {
		t.Fatal("should not announce while bodies lag")
	}
}

func TestShouldDeferTipBackfill(t *testing.T) {
	if !ShouldDeferTipBackfill(1_328_000, 1) {
		t.Fatal("large gap should defer")
	}
	if ShouldDeferTipBackfill(100, 99) {
		t.Fatal("small gap should not defer")
	}
}

func TestRangeHasMissingBlock(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(dir+"/h.bin", genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	missing, err := rangeHasMissingBlock(j, rs, 0, 0, 0)
	if err != nil || !missing {
		t.Fatalf("genesis body missing: missing=%v err=%v", missing, err)
	}
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	missing, err = rangeHasMissingBlock(j, rs, 0, 0, 0)
	if err != nil || missing {
		t.Fatalf("genesis stored: missing=%v err=%v", missing, err)
	}
}

func TestLaneForAddr(t *testing.T) {
	var s progressiveRawState
	s.SetSyncParallelism(7)
	if s.laneForAddr("1.2.3.4:22556") == 0 {
		t.Fatal("non-primary addr should not use lane 0")
	}
}

func TestShouldDeferInvBlockFetch(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(dir+"/h.bin", genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, genesisRaw[:80], 10_000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	genHash := pow.BlockHashLE(genesisRaw[:80])
	_ = rs.Put(genHash, genesisRaw)
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	bs.noteBlockStoredAt(0)
	const farHeight = int64(9000)
	h80, err := j.ReadHeaderAt(farHeight)
	if err != nil {
		t.Fatal(err)
	}
	farHash := pow.BlockHashLE(h80)
	farH, err := bs.Journal.HeightByBlockHashLE(farHash)
	if err != nil {
		t.Fatalf("height lookup: %v", err)
	}
	if farH != farHeight {
		t.Fatalf("far block height %d want %d (fake chain hash collision - adjust test heights)", farH, farHeight)
	}
	if !ShouldDeferInvBlockFetch(bs, farHash) {
		t.Fatalf("inv at height %d should defer (cont=%d)", farH, bs.ContiguousRawHeight())
	}
	h2, err := j.ReadHeaderAt(1)
	if err != nil {
		t.Fatal(err)
	}
	nearHash := pow.BlockHashLE(h2)
	if ShouldDeferInvBlockFetch(bs, nearHash) {
		t.Fatal("inv near frontier should not defer")
	}
	h500, err := j.ReadHeaderAt(500)
	if err != nil {
		t.Fatal(err)
	}
	if !ShouldDeferInvBlockFetch(bs, pow.BlockHashLE(h500)) {
		t.Fatal("inv far ahead of contiguous frontier should defer during forward IBD")
	}
}

func TestShouldSuppressInvTxFetchDuringIBD(t *testing.T) {
	if ShouldSuppressInvTxFetchDuringIBD(nil) {
		t.Fatal("nil store")
	}
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(dir+"/h.bin", genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, genesisRaw[:80], 600_000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	bs.noteBlockStoredAt(0)
	if !ShouldSuppressInvTxFetchDuringIBD(bs) {
		t.Fatal("headers far ahead of bodies must suppress mempool inv (not only tip<500k)")
	}
}

func TestForwardIBDStripeTip(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(dir+"/h.bin", genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = rs.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw)
	appendFakeHeaderChain(t, j, genesisRaw[:80], 2000)
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	bs.noteBlockStoredAt(0)
	tip, _ := j.TipHeight()
	hi := forwardIBDStripeTip(bs, 2, tip)
	want := int64(2 + blockDownloadWindow - 1)
	if hi != want {
		t.Fatalf("forward cap hi=%d want %d (Core BLOCK_DOWNLOAD_WINDOW)", hi, want)
	}
	lo, hi2, ok := syncStripeBounds(2, hi, 1, 3)
	if !ok || lo < 2 || hi2 > hi || lo > hi2 {
		t.Fatalf("assist stripe should stay inside forward window: %d..%d (window tip %d)", lo, hi2, hi)
	}
}

func TestSyncStripeBoundsGenesisTip(t *testing.T) {
	lo, hi, ok := syncStripeBounds(0, 0, 0, 1)
	if !ok || lo != 0 || hi != 0 {
		t.Fatalf("genesis stripe got %d..%d ok=%v", lo, hi, ok)
	}
}

func TestSyncStripeBounds(t *testing.T) {
	lo, hi, ok := syncStripeBounds(2, 101, 0, 4)
	if !ok || lo != 2 || hi != 26 {
		t.Fatalf("stripe0 got %d..%d ok=%v", lo, hi, ok)
	}
	lo, hi, ok = syncStripeBounds(2, 101, 3, 4)
	if !ok || lo != 77 || hi != 101 {
		t.Fatalf("stripe3 got %d..%d ok=%v", lo, hi, ok)
	}
}

func TestCapBodyDownloadTipDuringReplay(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(4985)
	bs := &BlockStoreCtx{Utxo: utxo, Params: p}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 3000
	bs.contiguousMu.Unlock()
	if got := capBodyDownloadTip(bs, 534_000); got != 4985 {
		t.Fatalf("download tip=%d want 4985 during replay", got)
	}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 4985
	bs.contiguousMu.Unlock()
	if got := capBodyDownloadTip(bs, 534_000); got != 534_000 {
		t.Fatalf("aligned tip=%d want header tip", got)
	}
}

func TestShouldPauseHeaderCatchUpForBodyIBD(t *testing.T) {
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
	bs.SeedContiguousTip(10_005)
	// No AssumeValid wired: legacy 500k pause still applies.
	if !ShouldPauseHeaderCatchUpForBodyIBD(bs, 6_000_000) {
		t.Fatal("deep body IBD should pause header catch-up")
	}
	if got := EffectiveConnectCatchUpMinLag(bs); got != connectCatchUpMinLagFrontier {
		t.Fatalf("deep body min lag=%d want %d (early ancient IBD)", got, connectCatchUpMinLagFrontier)
	}
	if got := PostBatchConnectLagThreshold(bs); got != connectCatchUpMinLagFrontier {
		t.Fatalf("post-batch threshold=%d want %d", got, connectCatchUpMinLagFrontier)
	}
	if ShouldRunHeaderAdvanceWatchdog(j, bs, 6_000_000) {
		t.Fatal("watchdog should not run during deep body IBD")
	}
}

func TestShouldPauseHeaderCatchUpForBodyIBDWaitsForAssumeValid(t *testing.T) {
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
	bs.SeedContiguousTip(10_005)
	bs.AssumeValid = consensus.NewAssumeValid("mainnet", "")
	if ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		t.Fatal("with default assumevalid unresolved, tip 534k must keep downloading headers toward 5.05M")
	}
	if got := headerBodyIBDPauseMinTip(bs); got != 5_050_000 {
		t.Fatalf("pause min tip=%d want 5050000", got)
	}
	// Pinning AV does not unlock pause early — local tip must still reach that height.
	bs.AssumeValid.PinResolvedHeight(5_050_000)
	if ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		t.Fatal("resolved assumevalid still requires tip >= AV height before pausing headers")
	}
	// Tip past a resolved AV height with a large body gap may pause getheaders.
	bs.AssumeValid = consensus.NewAssumeValid("mainnet", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	bs.AssumeValid.PinResolvedHeight(400_000)
	if got := headerBodyIBDPauseMinTip(bs); got != 400_000 {
		t.Fatalf("pause min tip=%d want 400000", got)
	}
	if !ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		t.Fatal("after tip passes assumevalid height, deep body IBD may pause header catch-up")
	}
}

func TestEffectiveProgressiveBatchSizeForIBDDeepBodyPause(t *testing.T) {
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
	got := EffectiveProgressiveBatchSizeForIBD(bs, 8)
	if got > 16 {
		t.Fatalf("batch size during body IBD pause got %d want <=16", got)
	}
	bs.SeedContiguousTip(3085)
	got2 := EffectiveProgressiveBatchSizeForIBD(bs, 6)
	if got2 != 16 {
		t.Fatalf("batch at height 3085 during pause got %d want 16 not 32", got2)
	}
}

func TestIdleFetchBatchesPerRound(t *testing.T) {
	if IdleFetchBatchesPerRound(nil) != 2 {
		t.Fatal("default idle batches")
	}
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
	if IdleFetchBatchesPerRound(bs) != 4 {
		t.Fatal("want 3 idle batches when header catch-up paused")
	}
}

func TestBlockFetchInvTypesNoWitness(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	types := blockFetchInvTypes(p)
	if len(types) != 1 || types[0] != wire.InvTypeBlock {
		t.Fatalf("inv types=%v want MSG_BLOCK only", types)
	}
}
