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

func TestEnsureBodyDownloadArmedClearsIdleFull(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 1000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	if err := EnsureLocalGenesis(bs); err != nil {
		t.Fatal(err)
	}
	bs.SeedContiguousTip(100)
	var raw progressiveRawState
	raw.mu.Lock()
	raw.idleFull = true
	raw.mu.Unlock()
	raw.ensureBodyDownloadArmed(bs)
	raw.mu.Lock()
	idle := raw.idleFull
	raw.mu.Unlock()
	if idle {
		t.Fatal("expected idleFull cleared while bodies lag headers")
	}
}

func TestMaybePumpBodyIBDRespectsInterval(t *testing.T) {
	var raw progressiveRawState
	raw.SetSyncParallelism(2)
	var last time.Time
	last = time.Now()
	n := MaybePumpBodyIBDDownload(nil, nil, chain.Params{}, nil, &raw, nil, nil, &last)
	if n != 0 {
		t.Fatalf("pump without blockstore returned %d", n)
	}
}

func TestBlockAssistSessionsStalled(t *testing.T) {
	raw := &progressiveRawState{}
	raw.mu.Lock()
	raw.lastStoredAt = time.Now().Add(-2 * time.Minute)
	raw.mu.Unlock()
	reg := NewAssistPeerRegistry()
	reg.Register("1.2.3.4:22556", 1)
	if !blockAssistSessionsStalled(raw, reg) {
		t.Fatal("expected stalled assist sessions")
	}
	raw.mu.Lock()
	raw.lastStoredAt = time.Now()
	raw.mu.Unlock()
	if blockAssistSessionsStalled(raw, reg) {
		t.Fatal("expected fresh body progress to clear stall")
	}
	raw.mu.Lock()
	raw.lastStoredAt = time.Now().Add(-2 * time.Minute)
	raw.inFlight = map[int64][32]byte{100: {}}
	raw.mu.Unlock()
	if blockAssistSessionsStalled(raw, reg) {
		t.Fatal("in-flight getdata must not relaunch assist workers")
	}
}

func TestMaybeRecoverIBDStallRunsWhenIdleFull(t *testing.T) {
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
	appendFakeHeaderChain(t, j, g80[:], 500)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	if err := EnsureLocalGenesis(bs); err != nil {
		t.Fatal(err)
	}
	bs.SeedContiguousTip(10)
	var raw progressiveRawState
	raw.mu.Lock()
	raw.idleFull = true
	raw.ibdStarted = time.Now().Add(-30 * time.Minute)
	raw.lastStoredAt = time.Now().Add(-20 * time.Minute)
	raw.blocksStoredIBD = 10
	raw.mu.Unlock()
	var last time.Time
	MaybeRecoverIBDStall(nil, nil, &raw, bs, nil, nil, nil, nil, nil, &last, nil, nil, nil)
	if last.IsZero() {
		t.Fatal("expected stall recovery despite idleFull when bodies lag headers")
	}
	raw.mu.Lock()
	idle := raw.idleFull
	raw.mu.Unlock()
	if idle {
		t.Fatal("expected ensureBodyDownloadArmed to clear idleFull during recovery")
	}
}

func TestReconcileHeaderCatchUpPendingArmsBodyDownload(t *testing.T) {
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
	bs.SeedContiguousTip(771)
	var pending atomic.Bool
	pending.Store(true)
	var raw progressiveRawState
	raw.mu.Lock()
	raw.idleFull = true
	raw.mu.Unlock()
	reconcileHeaderCatchUpPending(bs, &pending, &raw)
	raw.mu.Lock()
	idle := raw.idleFull
	raw.mu.Unlock()
	if idle {
		t.Fatal("expected body download armed after header deferral")
	}
}
