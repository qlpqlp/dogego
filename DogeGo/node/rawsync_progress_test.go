// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"path/filepath"
	"sync"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestProgressiveRawStateParallelClaim(t *testing.T) {
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
	for i := 0; i < 20; i++ {
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
	s.InitFromCheckpoint(dir, 20, -1)
	s.SetSyncParallelism(2)
	bs := &BlockStoreCtx{Journal: j, Raw: rs, Utxo: nil}

	b1, ok := s.claimBatch(bs, 0)
	if !ok || len(b1.heights) == 0 {
		t.Fatalf("claim1 ok=%v n=%d", ok, len(b1.heights))
	}
	b2, ok := s.claimBatch(bs, 1)
	if !ok || len(b2.heights) == 0 {
		t.Fatalf("claim2 ok=%v n=%d", ok, len(b2.heights))
	}
	for _, h := range b1.heights {
		for _, h2 := range b2.heights {
			if h == h2 {
				t.Fatalf("overlap height %d", h)
			}
		}
	}
	s.finishBatch(bs, b1, 0, nil)
	s.finishBatch(bs, b2, 0, nil)

	s.SetSyncParallelism(4)
	var wg sync.WaitGroup
	var mu sync.Mutex
	claimed := make(map[int64]struct{})
	for lane := 0; lane < 4; lane++ {
		lane := lane
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, ok := s.claimBatch(bs, lane)
			if !ok {
				return
			}
			mu.Lock()
			for _, h := range b.heights {
				if _, dup := claimed[h]; dup {
					mu.Unlock()
					t.Errorf("lane %d claimed duplicate height %d", lane, h)
					return
				}
				claimed[h] = struct{}{}
			}
			mu.Unlock()
			s.finishBatch(bs, b, 0, nil)
		}()
	}
	wg.Wait()
}

func TestClaimBatch_staleNextProbeStillFillsFromLowestMissing(t *testing.T) {
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
	for i := 0; i < 30; i++ {
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
	s.nextProbe = 25 // stale checkpoint ahead of the real gap at height 1
	s.SetSyncParallelism(3)
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	b, ok := s.claimBatch(bs, 0)
	if !ok || len(b.heights) == 0 {
		t.Fatalf("claim ok=%v n=%d", ok, len(b.heights))
	}
	if b.heights[0] != 1 {
		t.Fatalf("first claimed height %d want 1 (stale nextProbe must not skip the frontier)", b.heights[0])
	}
}

func TestClaimBatch_frontierFirstAllowsAssistLaneID(t *testing.T) {
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
	for i := 0; i < 600; i++ {
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
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	var s progressiveRawState
	s.SetSyncParallelism(4)
	bs := &BlockStoreCtx{Journal: j, Raw: rs, Params: p}
	b, ok := s.claimBatch(bs, 2)
	if !ok || len(b.heights) == 0 {
		t.Fatalf("assist lane claim ok=%v n=%d", ok, len(b.heights))
	}
	if b.heights[0] != 1 {
		t.Fatalf("first height %d want 1 (frontier-first, not stripe 2)", b.heights[0])
	}
}

func TestClaimBatch_frontierFirstPipelinesNextLane(t *testing.T) {
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
	for i := 0; i < 600; i++ {
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
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	var s progressiveRawState
	s.SetSyncParallelism(4)
	bs := &BlockStoreCtx{Journal: j, Raw: rs, Params: p}
	a, ok := s.claimBatch(bs, 0)
	if !ok || len(a.heights) == 0 {
		t.Fatalf("lane 0 claim ok=%v n=%d", ok, len(a.heights))
	}
	b, ok := s.claimBatch(bs, 2)
	if !ok || len(b.heights) == 0 {
		t.Fatalf("lane 2 should claim a disjoint ahead chunk, ok=%v n=%d", ok, len(b.heights))
	}
	for _, h := range b.heights {
		if h >= a.lo && h <= a.hi {
			t.Fatalf("lane 2 height %d overlaps lane 0 %d..%d", h, a.lo, a.hi)
		}
	}
	if b.heights[0] <= a.lo {
		t.Fatalf("lane 2 first height %d should be ahead of lane 0 start %d", b.heights[0], a.lo)
	}
}

func TestClaimBatch_concurrentLanesGetDistinctRanges(t *testing.T) {
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
	for i := 0; i < 600; i++ {
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
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	var s progressiveRawState
	s.SetSyncParallelism(4)
	bs := &BlockStoreCtx{Journal: j, Raw: rs, Params: p}
	type res struct {
		c  rawBatchClaim
		ok bool
	}
	out := make([]res, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); out[0].c, out[0].ok = s.claimBatch(bs, 0) }()
	go func() { defer wg.Done(); out[1].c, out[1].ok = s.claimBatch(bs, 2) }()
	wg.Wait()
	if !out[0].ok || !out[1].ok {
		t.Fatalf("concurrent claims ok=%v/%v (race used to leave the second lane empty)", out[0].ok, out[1].ok)
	}
	seen := map[int64]int{}
	for i, r := range out {
		for _, h := range r.c.heights {
			if prev, dup := seen[h]; dup {
				t.Fatalf("height %d claimed by lane %d and %d", h, prev, i)
			}
			seen[h] = i
		}
	}
	if len(seen) < 2 {
		t.Fatalf("want both lanes to hold heights, got %d unique", len(seen))
	}
}

func TestPrepareAtStartup_claimsGenesisWhenMissing(t *testing.T) {
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
	var s progressiveRawState
	bs := &BlockStoreCtx{Journal: j, Raw: rs}
	s.PrepareAtStartup(bs)
	if s.nextProbe != 0 {
		t.Fatalf("nextProbe %d want 0 (genesis missing)", s.nextProbe)
	}
	b, ok := s.claimBatch(bs, 0)
	if !ok || len(b.heights) == 0 || b.heights[0] != 0 {
		t.Fatalf("claim genesis ok=%v heights=%v", ok, b.heights)
	}
}

func TestClaimBatch_skipsStoredHolesInFrontierWindow(t *testing.T) {
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
	for i := 0; i < 80; i++ {
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
	putAt := func(height int64) {
		t.Helper()
		hdr, err := j.ReadHeaderAt(height)
		if err != nil {
			t.Fatal(err)
		}
		raw := store.MakeTestBlockRaw(t, hdr)
		if err := rs.Put(pow.BlockHashLE(hdr), raw); err != nil {
			t.Fatal(err)
		}
	}
	putAt(0)
	putAt(2)
	putAt(4)
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	var s progressiveRawState
	bs := &BlockStoreCtx{Journal: j, Raw: rs, Params: p}
	bs.noteBlockStoredAt(0)
	b, ok := s.claimBatch(bs, 0)
	if !ok || len(b.heights) < 3 {
		t.Fatalf("claim ok=%v n=%d want several hole heights, got %v", ok, len(b.heights), b.heights)
	}
	if b.heights[0] != 1 {
		t.Fatalf("first claimed %d want 1 (lowest hole)", b.heights[0])
	}
	for i := 1; i < len(b.heights); i++ {
		if b.heights[i] <= b.heights[i-1] {
			t.Fatalf("heights not increasing: %v", b.heights)
		}
		if b.heights[i]%2 == 0 && b.heights[i] <= 4 {
			t.Fatalf("claimed already-stored height %d", b.heights[i])
		}
	}
	want := map[int64]bool{1: true, 3: true, 5: true}
	for h := range want {
		found := false
		for _, got := range b.heights {
			if got == h {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing hole %d in claim %v", h, b.heights)
		}
	}
}

func TestClaimBatch_skipsBusyMiddleKeepsLaterHeights(t *testing.T) {
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
	for i := 0; i < 80; i++ {
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
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	var s progressiveRawState
	s.SetSyncParallelism(2)
	bs := &BlockStoreCtx{Journal: j, Raw: rs, Params: p}
	bs.noteBlockStoredAt(0)
	s.inFlight = map[int64][32]byte{2: {}}
	s.inFlightLane = map[int64]int{2: 0}
	b, ok := s.claimBatch(bs, 0)
	if !ok || len(b.heights) < 2 {
		t.Fatalf("claim ok=%v n=%d heights=%v", ok, len(b.heights), b.heights)
	}
	for _, h := range b.heights {
		if h == 2 {
			t.Fatalf("claimed in-flight height 2: %v", b.heights)
		}
	}
	if b.heights[0] != 1 {
		t.Fatalf("first %d want 1", b.heights[0])
	}
	foundLater := false
	for _, h := range b.heights {
		if h > 2 {
			foundLater = true
			break
		}
	}
	if !foundLater {
		t.Fatalf("truncated at busy height 2 instead of claiming later holes: %v", b.heights)
	}
}

func TestClaimBatch_freeHoleNotSkippedByAheadInFlight(t *testing.T) {
	// Regression: claimBatch used to set lowMissing = min(inFlight). With the contiguous
	// hole free and only ahead heights claimed, that skipped the hole forever.
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
	appendFakeHeaderChain(t, j, g80[:], 4000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	bs.SeedContiguousTip(1000)
	var s progressiveRawState
	s.SetSyncParallelism(8)
	s.inFlight = make(map[int64][32]byte)
	s.inFlightLane = make(map[int64]int)
	// Ahead claims only — hole at 1001 is free.
	for h := int64(1002); h <= 1002+200; h++ {
		s.inFlight[h] = [32]byte{}
		s.inFlightLane[h] = 1
	}
	b, ok := s.claimBatch(bs, 0)
	if !ok {
		t.Fatal("expected claim of free contiguous hole")
	}
	if b.heights[0] != 1001 {
		t.Fatalf("first height %d want 1001 (hole must not be skipped for min ahead in-flight)", b.heights[0])
	}
}

func TestClaimBatch_prunesStaleInFlightBelowContiguous(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 3000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	bs.SeedContiguousTip(2000)
	var s progressiveRawState
	s.SetSyncParallelism(4)
	s.inFlight = map[int64][32]byte{50: {}, 100: {}, 2001: {}}
	s.inFlightLane = map[int64]int{50: 2, 100: 2, 2001: 3}
	b, ok := s.claimBatch(bs, 0)
	if !ok {
		t.Fatal("expected claim")
	}
	if _, ok := s.inFlight[50]; ok {
		t.Fatal("stale in-flight below contiguous must be pruned")
	}
	if _, ok := s.inFlight[100]; ok {
		t.Fatal("stale in-flight below contiguous must be pruned")
	}
	if b.heights[0] != 2001 && b.heights[0] != 2002 {
		// 2001 may still be held by another lane; first free hole is then 2002.
		if _, busy := s.inFlight[2001]; busy && b.heights[0] == 2002 {
			return
		}
		t.Fatalf("first height %d; want frontier near 2001", b.heights[0])
	}
}
