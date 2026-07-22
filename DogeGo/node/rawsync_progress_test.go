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
