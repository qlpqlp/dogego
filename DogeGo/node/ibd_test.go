// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

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
	if g := EffectiveBlockSyncWorkersOpt(8, 0, true); g != 7 {
		t.Fatalf("ibd optimize auto from maxoutbound 8: got %d want 7 (outbound-1)", g)
	}
	if g := EffectiveBlockSyncWorkersOpt(3, 0, true); g != 3 {
		t.Fatalf("ibd optimize respects tiny outbound: got %d want 3", g)
	}
	if g := EffectiveBlockSyncWorkersOpt(32, 0, true); g != maxBlockAssistWorkers {
		t.Fatalf("ibd optimize full pool at outbound 32: got %d want %d", g, maxBlockAssistWorkers)
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
	a := s.laneForAddr("1.2.3.4:22556")
	b := s.laneForAddr("5.6.7.8:22556")
	if a == 0 || b == 0 {
		t.Fatal("non-primary addr should not use lane 0")
	}
	if a == b {
		t.Fatalf("distinct peers must not share a getdata lane (got %d)", a)
	}
	if s.laneForAddr("1.2.3.4:22556") != a {
		t.Fatal("lane assignment must be stable for a peer")
	}
	if a < 7 || b < 7 {
		t.Fatalf("relay lanes %d,%d must start at syncWorkers=7 (assist reserves 1-6)", a, b)
	}
	if s.syncWorkers != 7 {
		t.Fatalf("relay lanes must not grow syncWorkers (got %d)", s.syncWorkers)
	}
}

func TestClaimBatchDoesNotGrowSyncWorkersForRelayLane(t *testing.T) {
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
	for i := 0; i < 40; i++ {
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
	var s progressiveRawState
	s.SetSyncParallelism(8)
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	bs.noteBlockStoredAt(0)
	if _, ok := s.claimBatch(bs, 52); !ok {
		t.Fatal("relay lane should still claim via the shared frontier window")
	}
	if s.syncWorkers != 8 {
		t.Fatalf("claimBatch grew syncWorkers to %d (live node hit 53 lanes and 1h timeouts)", s.syncWorkers)
	}
}

func TestLaneForAddrPoolExhausted(t *testing.T) {
	var s progressiveRawState
	s.SetSyncParallelism(2)
	for i := 0; i < defaultMaxOutbound; i++ {
		if s.laneForAddr(fmt.Sprintf("10.0.0.%d:22556", i+1)) < 2 {
			t.Fatal("expected a relay lane")
		}
	}
	if s.laneForAddr("10.9.9.9:22556") != -1 {
		t.Fatal("relay lane pool must be bounded")
	}
	if s.syncWorkers != 2 {
		t.Fatalf("syncWorkers grew to %d", s.syncWorkers)
	}
}

func TestReleaseOrphanInFlight(t *testing.T) {
	s := &progressiveRawState{
		inFlight:     map[int64][32]byte{10: {}, 11: {}, 12: {}},
		inFlightLane: map[int64]int{10: 9, 11: 9, 12: 1},
		laneAddr:     map[int]string{1: "1.2.3.4:22556"},
		activeBatch:  map[int]*batchSlot{1: {}},
	}
	if n := s.releaseOrphanInFlight(); n != 2 {
		t.Fatalf("freed %d want 2 (lane 9 has no live peer)", n)
	}
	if _, ok := s.inFlight[12]; !ok {
		t.Fatal("live lane 1 claim must stay")
	}
}

func TestReleaseOrphanInFlightClearsDeadWindow(t *testing.T) {
	s := &progressiveRawState{
		inFlight:     map[int64][32]byte{10: {}, 11: {}, 12: {}},
		inFlightLane: map[int64]int{},
	}
	if n := s.releaseOrphanInFlight(); n != 3 {
		t.Fatalf("freed %d want 3 (no live peer, window jammed)", n)
	}
	if len(s.inFlight) != 0 {
		t.Fatalf("inFlight leftover %v", s.inFlight)
	}
}

func TestStartBatchDoesNotCancelInFlight(t *testing.T) {
	var s progressiveRawState
	parent := context.Background()
	ctx1, end1, ok := s.startBatch(1, parent, time.Minute)
	if !ok || ctx1 == nil {
		t.Fatal("first startBatch should start")
	}
	defer end1()
	_, _, ok2 := s.startBatch(1, parent, time.Minute)
	if ok2 {
		t.Fatal("second startBatch on same lane must not cancel the first")
	}
	select {
	case <-ctx1.Done():
		t.Fatal("first batch context was canceled")
	default:
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
	// Frontier-first IBD uses the full body fetch window (up to maxIBDFetchWindow),
	// which past a short tip equals the header tip.
	want := tip
	if hi != want {
		t.Fatalf("forward cap hi=%d want %d (shared-window IBD past the hole)", hi, want)
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

func TestSyncBatchChunkBoundsDisjoint(t *testing.T) {
	const batch = 1000
	const workers = 4
	low, tip := int64(100), int64(100+workers*batch-1)
	seen := map[int64]int{}
	for w := 0; w < workers; w++ {
		lo, hi, ok := syncBatchChunkBounds(low, tip, w, workers, batch, 0)
		if !ok {
			t.Fatalf("worker %d: expected chunk", w)
		}
		wantLo := low + int64(w)*batch
		wantHi := wantLo + batch - 1
		if lo != wantLo || hi != wantHi {
			t.Fatalf("worker %d got %d..%d want %d..%d", w, lo, hi, wantLo, wantHi)
		}
		for h := lo; h <= hi; h++ {
			if prev, clash := seen[h]; clash {
				t.Fatalf("height %d claimed by lanes %d and %d", h, prev, w)
			}
			seen[h] = w
		}
	}
	// After finishing first chunk, lane 0 takes the next free 1000 (slot 1).
	lo, hi, ok := syncBatchChunkBounds(low, tip+batch, 0, workers, batch, 1)
	if !ok || lo != low+int64(workers)*batch || hi != lo+batch-1 {
		t.Fatalf("lane0 refill chunk got %d..%d ok=%v", lo, hi, ok)
	}
}

func TestChunkLaneForWorker(t *testing.T) {
	if got := chunkLaneForWorker(0, 6); got != 0 {
		t.Fatalf("lane0=%d", got)
	}
	if got := chunkLaneForWorker(5, 6); got != 5 {
		t.Fatalf("assist=%d", got)
	}
	if got := chunkLaneForWorker(6, 6); got < 1 || got > 5 {
		t.Fatalf("relay mapped to ahead lane, got %d", got)
	}
	if got := chunkLaneForWorker(7, 6); got < 1 || got > 5 {
		t.Fatalf("relay2 mapped to ahead lane, got %d", got)
	}
}

func TestMayClaimContiguousHoleSoftOpen(t *testing.T) {
	s := &progressiveRawState{
		laneAddr: map[int]string{
			0: "1.2.3.4:22556",
			1: "5.6.7.8:22556",
		},
		softStallFrontier: 100,
		softStallPeer:     "1.2.3.4:22556",
		laneDownloadSince: map[int]time.Time{0: time.Now()},
	}
	inFlight := map[int64][32]byte{}
	if s.mayClaimContiguousHole(nil, 0, 6, 100, inFlight) {
		t.Fatal("soft-stall peer must not re-claim the hole")
	}
	if !s.mayClaimContiguousHole(nil, 1, 6, 100, inFlight) {
		t.Fatal("assist must reclaim hole after soft-stall")
	}
	s.softStallFrontier = -1
	s.softStallPeer = ""
	if s.mayClaimContiguousHole(nil, 1, 6, 100, inFlight) {
		t.Fatal("assist must not steal hole while lane0 alive+active and no soft-stall")
	}
	delete(s.laneDownloadSince, 0)
	if !s.mayClaimContiguousHole(nil, 1, 6, 100, inFlight) {
		t.Fatal("assist must reclaim hole when lane0 is idle")
	}
	if !s.mayClaimContiguousHole(nil, 0, 6, 100, inFlight) {
		t.Fatal("lane0 owns hole")
	}
}

func TestHoleFillBatchSize(t *testing.T) {
	s := &progressiveRawState{softStallFrontier: -1}
	if got := s.holeFillBatchSize(100, ibdGetDataBatch); got != 64 {
		t.Fatalf("hole batch=%d want 64 (deep IBD uses peer budget, not 16-wide Core hole)", got)
	}
	s.softStallFrontier = 100
	if got := s.holeFillBatchSize(100, ibdGetDataBatch); got != 64 {
		t.Fatalf("soft-stall hole batch=%d want 64", got)
	}
	if got := s.holeFillBatchSize(200, 8); got != 8 {
		t.Fatalf("hole batch must not exceed peer budget, got %d", got)
	}
}

func TestShouldRunDedicatedHeaderDespiteBodyPause(t *testing.T) {
	if ShouldRunDedicatedHeaderDespiteBodyPause(nil, 100) {
		t.Fatal("nil")
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
	if !ShouldDeferConnectForBodyDownload(bs) {
		t.Fatal("deep body IBD must defer ConnectBlock until bodies catch headers")
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
	// Pinning AV does not unlock pause early â€” local tip must still reach that height.
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
	if got != ibdPeerInFlightInitial {
		t.Fatalf("batch size during download-first body IBD got %d want %d (adaptive start)", got, ibdPeerInFlightInitial)
	}
	bs.SeedContiguousTip(3085)
	got2 := EffectiveProgressiveBatchSizeForIBD(bs, 6)
	if got2 != ibdPeerInFlightInitial {
		t.Fatalf("batch at height 3085 during download-first got %d want %d", got2, ibdPeerInFlightInitial)
	}
}

func TestShouldRefillGetData(t *testing.T) {
	if shouldRefillGetData(minInFlightBlocks) {
		t.Fatal("at minInFlightBlocks must not refill")
	}
	if !shouldRefillGetData(minInFlightBlocks - 1) {
		t.Fatal("below minInFlightBlocks must refill")
	}
	if !shouldRefillGetData(0) {
		t.Fatal("empty in-flight queue must refill like ltcd")
	}
}

func TestGetdataRefillThresholdFatBatch(t *testing.T) {
	wantCore := (progressiveBatchSize * 3) / 4
	if getdataRefillThreshold(progressiveBatchSize) != wantCore {
		t.Fatalf("Core-sized batch threshold=%d want %d (3/4 of 16)", getdataRefillThreshold(progressiveBatchSize), wantCore)
	}
	if shouldRefillGetDataAt(wantCore, progressiveBatchSize) {
		t.Fatal("at Core refill threshold must not refill")
	}
	if !shouldRefillGetDataAt(wantCore-1, progressiveBatchSize) {
		t.Fatal("below Core refill threshold must refill to hide getdata RTT")
	}
	if getdataRefillThreshold(ibdGetDataBatch) != ibdGetDataBatch/4 {
		t.Fatalf("max-peer-budget threshold=%d want %d", getdataRefillThreshold(ibdGetDataBatch), ibdGetDataBatch/4)
	}
	if getdataRefillThreshold(256) != 64 {
		t.Fatalf("256-inv threshold=%d want 64 (refill at quarter-full)", getdataRefillThreshold(256))
	}
	if getdataRefillThreshold(ibdPeerInFlightFast) != ibdPeerInFlightFast/4 {
		t.Fatalf("fast-peer threshold=%d want %d", getdataRefillThreshold(ibdPeerInFlightFast), ibdPeerInFlightFast/4)
	}
}

func TestIBDBodyFetchWindowKeepsAllPeersBusy(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 600_000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	bs.SeedContiguousTip(55_657)
	const workers = 12
	win := ibdBodyFetchWindow(bs, workers)
	if win != int64(maxIBDFetchWindow) {
		t.Fatalf("fetch window=%d want %d (global adaptive IBD cap)", win, maxIBDFetchWindow)
	}
	inflated := ibdBodyFetchWindow(bs, 125)
	if inflated != int64(maxIBDFetchWindow) {
		t.Fatalf("inflated lane count window=%d want %d", inflated, maxIBDFetchWindow)
	}
	hi := forwardIBDStripeTipFor(bs, 55_658, 600_000, workers)
	if hi != 55_658+win-1 {
		t.Fatalf("stripe tip=%d want %d", hi, 55_658+win-1)
	}
}

func TestApplyGetDataRefillSkipsWhenAboveMin(t *testing.T) {
	pending := make(map[[32]byte]struct{}, minInFlightBlocks)
	for i := 0; i < minInFlightBlocks; i++ {
		pending[[32]byte{byte(i)}] = struct{}{}
	}
	called := 0
	hooks := &getdataBatchHooks{Refill: func(int) ([][32]byte, []int64) {
		called++
		return [][32]byte{{9}}, []int64{1}
	}}
	n, err := applyGetDataRefill(&MsgWriter{}, pending, map[[32]byte]int64{}, wire.InvTypeBlock, hooks)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || called != 0 {
		t.Fatalf("refill ran with a full pipe n=%d called=%d", n, called)
	}
}

func TestMergeRawBatchClaims(t *testing.T) {
	base := rawBatchClaim{lo: 10, hi: 20, heights: []int64{10, 20}, hashes: [][32]byte{{1}, {2}}}
	extra := []rawBatchClaim{{lo: 21, hi: 25, heights: []int64{21, 25}, hashes: [][32]byte{{3}, {4}}}}
	got := mergeRawBatchClaims(base, extra)
	if got.hi != 25 || len(got.heights) != 4 || len(got.hashes) != 4 {
		t.Fatalf("merged claim %+v", got)
	}
}

func TestReleaseInFlightHeight(t *testing.T) {
	s := &progressiveRawState{
		inFlight:     map[int64][32]byte{100: {}, 101: {}},
		inFlightLane: map[int64]int{100: 0, 101: 0},
	}
	s.releaseInFlightHeight(100)
	if _, ok := s.inFlight[100]; ok {
		t.Fatal("height 100 still in flight")
	}
	if _, ok := s.inFlight[101]; !ok {
		t.Fatal("height 101 should remain in flight")
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
	if IdleFetchBatchesPerRound(bs) != 8 {
		t.Fatal("want 8 idle batches when header catch-up paused")
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

func TestShouldDeferConnectForBodyDownload(t *testing.T) {
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
	bs.SeedContiguousTip(54_506)
	if !ShouldDeferConnectForBodyDownload(bs) {
		t.Fatal("headers far ahead of bodies must defer connect")
	}
	bs.SeedContiguousTip(533_500)
	if ShouldDeferConnectForBodyDownload(bs) {
		t.Fatal("bodies within BLOCK_DOWNLOAD_WINDOW of header tip should connect")
	}
	if ShouldDeferConnectForBodyDownload(nil) {
		t.Fatal("nil store must not defer")
	}
}

func TestClaimBatchAdaptiveDownloadFirstIBD(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 5000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	bs.SeedContiguousTip(2000)
	if !shouldSkipDiskBodyProbe(bs) {
		t.Fatal("download-first IBD must skip per-height disk probes")
	}
	var s progressiveRawState
	s.SetSyncParallelism(8)
	b, ok := s.claimBatch(bs, 0)
	if !ok {
		t.Fatal("expected claim")
	}
	// Deep IBD shared-window floor is 64 for small/mid bodies (Core 16 under-fills multi-peer).
	want := ibdDeepBodyStartBudget(bs)
	if len(b.heights) != want {
		t.Fatalf("claimed %d want %d (deep IBD start budget)", len(b.heights), want)
	}
	if b.heights[0] != 2001 {
		t.Fatalf("first height %d want 2001", b.heights[0])
	}
	if b.heights[len(b.heights)-1] != 2000+int64(want) {
		t.Fatalf("last height %d want %d", b.heights[len(b.heights)-1], 2000+int64(want))
	}
}

func TestPeerInFlightBudget(t *testing.T) {
	s := &progressiveRawState{
		syncWorkers: 8,
		laneAddr:    map[int]string{0: "1.2.3.4:22556", 1: "5.6.7.8:22556"},
	}
	if got := s.peerInFlightBudget(nil, 0); got != ibdPeerInFlightInitial {
		t.Fatalf("unknown peer budget=%d want %d", got, ibdPeerInFlightInitial)
	}
	s.mu.Lock()
	s.lastStallPeer = "1.2.3.4:22556"
	s.lastStallAt = time.Now()
	s.mu.Unlock()
	if got := s.peerInFlightBudget(nil, 0); got != ibdPeerInFlightSlowFloor {
		t.Fatalf("hard-stall peer budget=%d want %d", got, ibdPeerInFlightSlowFloor)
	}
	if got := s.peerInFlightBudget(nil, 1); got != ibdPeerInFlightInitial {
		t.Fatalf("other peer should stay initial, got %d", got)
	}
	s.mu.Lock()
	s.lastStallPeer = ""
	s.lastStallAt = time.Time{}
	s.laneBudgetApplied = nil
	now := time.Now()
	// ~25 blk/sec over the window → climb toward max (asymmetric: one step per call).
	s.laneDelivery = map[int][]laneDeliverySample{
		1: {{at: now.Add(-2 * time.Second), n: 50}},
	}
	s.mu.Unlock()
	got := s.peerInFlightBudget(nil, 1)
	if got <= ibdPeerInFlightInitial {
		t.Fatalf("fast peer should ramp above initial, got %d", got)
	}
	for i := 0; i < 8; i++ {
		got = s.peerInFlightBudget(nil, 1)
	}
	if got != ibdPeerInFlightMax {
		t.Fatalf("fast peer budget after ramp=%d want %d", got, ibdPeerInFlightMax)
	}
	s.mu.Lock()
	s.laneDelivery[1] = []laneDeliverySample{{at: now.Add(-10 * time.Second), n: 5}}
	s.mu.Unlock()
	got = s.peerInFlightBudget(nil, 1)
	if got < ibdPeerInFlightInitial {
		t.Fatalf("rate-based path must not floor below initial, got %d", got)
	}
}

func TestShouldUseParallelBatchChunksDefersToSharedWindow(t *testing.T) {
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
	bs.SeedContiguousTip(5000)
	low := int64(5001)
	if !shouldFillContiguousFrontierFirst(bs, low) {
		t.Fatal("expected frontier-first during deep body IBD")
	}
	if shouldUseParallelBatchChunks(bs, low) {
		t.Fatal("exclusive chunks must yield to Core shared window during frontier-first IBD")
	}
}

func TestPeerInFlightBudgetReprobesInitialAfterHardFloor(t *testing.T) {
	s := &progressiveRawState{
		syncWorkers:       8,
		laneAddr:          map[int]string{0: "1.2.3.4:22556"},
		laneBudgetApplied: map[int]int{0: ibdPeerInFlightSlowFloor},
	}
	if got := s.peerInFlightBudget(nil, 0); got != ibdPeerInFlightInitial {
		t.Fatalf("stale slow-floor mark must jump to initial, got %d", got)
	}
}

func TestSoftStallDoesNotFloorBudget(t *testing.T) {
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
	bs.SeedContiguousTip(5000)
	if !ShouldDeferConnectForBodyDownload(bs) {
		t.Fatal("need deep body IBD for soft-stall path")
	}
	s := &progressiveRawState{
		syncWorkers:       8,
		inFlight:          map[int64][32]byte{5001: {}, 5100: {}},
		inFlightLane:      map[int64]int{5001: 0, 5100: 0},
		laneAddr:          map[int]string{0: "1.2.3.4:22556"},
		laneDownloadSince: map[int]time.Time{0: time.Now()},
		stallingSince:     time.Now().Add(-20 * time.Second),
		softStallFrontier: -1,
		laneDelivery: map[int][]laneDeliverySample{
			0: {{at: time.Now().Add(-2 * time.Second), n: 40}},
		},
	}
	peer, stalled := s.maybePenalizeStallingPeer(bs, NewBlockPeerScorer(), nil)
	if !stalled || peer != "" {
		t.Fatalf("expected soft-stall (no disconnect), peer=%q stalled=%v", peer, stalled)
	}
	if s.lastStallPeer != "" {
		t.Fatal("soft-stall must not set lastStallPeer (budget floor)")
	}
	if s.softStallPeer != "1.2.3.4:22556" {
		t.Fatalf("softStallPeer=%q", s.softStallPeer)
	}
	if _, ok := s.inFlight[5001]; ok {
		t.Fatal("frontier should be released")
	}
	if _, ok := s.inFlight[5100]; !ok {
		t.Fatal("ahead claims must be kept")
	}
	got := s.peerInFlightBudget(bs, 0)
	if got <= ibdPeerInFlightSlow {
		t.Fatalf("soft-stall must not floor budget, got %d", got)
	}
}

func TestTrimLaneInFlightToBudget(t *testing.T) {
	s := &progressiveRawState{
		inFlight: map[int64][32]byte{
			100: {}, 101: {}, 102: {}, 103: {}, 200: {},
		},
		inFlightLane: map[int64]int{
			100: 0, 101: 0, 102: 0, 103: 0, 200: 0,
		},
	}
	s.mu.Lock()
	freed := s.trimLaneInFlightToBudgetLocked(0, 2)
	s.mu.Unlock()
	if freed != 3 {
		t.Fatalf("freed=%d want 3", freed)
	}
	if len(s.inFlight) != 2 {
		t.Fatalf("remaining=%d want 2 (lowest heights kept)", len(s.inFlight))
	}
	if _, ok := s.inFlight[100]; !ok {
		t.Fatal("keep lowest height 100")
	}
	if _, ok := s.inFlight[101]; !ok {
		t.Fatal("keep height 101")
	}
}

func TestSoftStallEscalatesToHard(t *testing.T) {
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
	bs.SeedContiguousTip(5000)
	scorer := NewBlockPeerScorer()
	s := &progressiveRawState{
		syncWorkers:       4,
		laneAddr:          map[int]string{0: "1.2.3.4:22556"},
		softStallFrontier: -1,
	}
	for i := 0; i < softStallEscalateCount; i++ {
		s.inFlight = map[int64][32]byte{5001: {}, 5100: {}}
		s.inFlightLane = map[int64]int{5001: 0, 5100: 0}
		s.laneDownloadSince = map[int]time.Time{0: time.Now()}
		s.stallingSince = time.Now().Add(-20 * time.Second)
		peer, stalled := s.maybePenalizeStallingPeer(bs, scorer, nil)
		if !stalled || peer != "" {
			t.Fatalf("soft %d: want soft-stall, peer=%q stalled=%v", i+1, peer, stalled)
		}
	}
	s.inFlight = map[int64][32]byte{5001: {}, 5100: {}}
	s.inFlightLane = map[int64]int{5001: 0, 5100: 0}
	s.laneDownloadSince = map[int]time.Time{0: time.Now()}
	s.stallingSince = time.Now().Add(-20 * time.Second)
	peer, stalled := s.maybePenalizeStallingPeer(bs, scorer, nil)
	if !stalled || peer != "1.2.3.4:22556" {
		t.Fatalf("after %d softs want hard disconnect, peer=%q stalled=%v", softStallEscalateCount, peer, stalled)
	}
	if len(s.inFlight) != 0 {
		t.Fatalf("hard stall must free lane, left %v", s.inFlight)
	}
}
