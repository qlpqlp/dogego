// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"sync/atomic"
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestReconcileHeaderCatchUpPendingClearsDuringDeepBodyIBD(t *testing.T) {
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
	bs.SeedContiguousTip(17_807)
	var pending atomic.Bool
	pending.Store(true)
	reconcileHeaderCatchUpPending(bs, &pending, nil)
	if pending.Load() {
		t.Fatal("expected headerCatchUpPending cleared when body IBD owns pipeline")
	}
}

func TestInitProgressiveRawAtStartupRealignsProbe(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 100)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	if err := EnsureLocalGenesis(bs); err != nil {
		t.Fatal(err)
	}
	bs.SeedContiguousTip(50)
	if err := store.SaveRawBlockSyncCheckpoint(dir, store.RawBlockSyncCheckpoint{NextProbeHeight: 10}); err != nil {
		t.Fatal(err)
	}
	var s progressiveRawState
	s.initProgressiveRawAtStartup(dir, bs, 4)
	s.mu.Lock()
	probe := s.nextProbe
	s.mu.Unlock()
	if probe != 51 {
		t.Fatalf("nextProbe=%d want 51 (contiguous+1)", probe)
	}
}

func TestTryHeaderSyncRecoveryPassSkipsDuringDeepBodyIBD(t *testing.T) {
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
	bs.SeedContiguousTip(616)
	ok, peer, err := tryHeaderSyncRecoveryPass(HeaderSyncRecoveryEnv{
		Journal: j, BlockStore: bs,
	}, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ok || peer != nil {
		t.Fatalf("want skip during body IBD, got ok=%v peer=%v", ok, peer)
	}
}

func TestMaybeResumeHeaderCatchUpAfterBodyIBDPauseLifted(t *testing.T) {
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
	bs.SeedContiguousTip(484_001) // gap 49999 - body pause lifts
	wasPaused := true
	var pending atomic.Bool
	var lastKick time.Time
	kicked := false
	ok := MaybeResumeHeaderCatchUpAfterBodyIBD(j, bs, 6_000_000, &wasPaused, &pending, &lastKick, func(force bool) bool {
		kicked = true
		return true
	})
	if !ok || !kicked || !pending.Load() {
		t.Fatalf("resume ok=%v kicked=%v pending=%v", ok, kicked, pending.Load())
	}
	if wasPaused {
		t.Fatal("expected wasBodyIBDPaused cleared after pause lift")
	}
}

func TestMaybeResumeHeaderCatchUpSkipsDuringDeepBodyIBD(t *testing.T) {
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
	bs.SeedContiguousTip(10_000)
	wasPaused := true
	var pending atomic.Bool
	kicked := false
	ok := MaybeResumeHeaderCatchUpAfterBodyIBD(j, bs, 6_000_000, &wasPaused, &pending, nil, func(force bool) bool {
		kicked = true
		return true
	})
	if ok || kicked || pending.Load() {
		t.Fatalf("deep body IBD should not resume headers: ok=%v kicked=%v pending=%v", ok, kicked, pending.Load())
	}
	if !wasPaused {
		t.Fatal("expected wasBodyIBDPaused still true during deep body IBD")
	}
}
