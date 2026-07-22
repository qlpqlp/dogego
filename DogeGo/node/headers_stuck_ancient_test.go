// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// appendHeaderRange writes heights [from, to] in one batch (same path as P2P header sync).
func appendHeaderRange(t *testing.T, j *store.HeaderJournal, genesis80 []byte, from, to int64, timeAt func(int64) uint32) {
	t.Helper()
	if from > to {
		return
	}
	n := int(to - from + 1)
	batch := make([]byte, 80*n)
	for h := from; h <= to; h++ {
		h80 := make([]byte, 80)
		copy(h80, genesis80)
		binaryLETime(h80, timeAt(h))
		off := int(h-from) * 80
		copy(batch[off:off+80], h80)
	}
	if err := j.AppendWireHeaderBatch(batch); err != nil {
		t.Fatal(err)
	}
}

func TestMaybeResetStuckAncientHeaderChain_ancientTipNoBodies(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(dir+"/h.bin", g80[:])
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, nil, nil, nil)
	// Fill headers with 2014-era block times (partial mainnet IBD scenario).
	appendHeaderRange(t, j, g80[:], 1, 8000, func(h int64) uint32 {
		return 1_400_000_000 + uint32(h)*60
	})
	reset, err := MaybeResetStuckAncientHeaderChain(j, nil, p, bs, 6_000_000)
	if !reset || err == nil {
		t.Fatalf("reset=%v err=%v want genesis reset", reset, err)
	}
	tip, _ := j.TipHeight()
	if tip != 0 {
		t.Fatalf("tip=%d want 0", tip)
	}
}

func TestMaybeResetStuckAncientHeaderChain_skipsOnTestnet(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(dir+"/h.bin", g80[:])
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, nil, nil, nil)
	appendHeaderRange(t, j, g80[:], 1, 500, func(int64) uint32 { return 1_400_000_000 })
	reset, err := MaybeResetStuckAncientHeaderChain(j, nil, p, bs, 6_000_000)
	if reset {
		t.Fatalf("unexpected reset on testnet: err=%v", err)
	}
}

func TestRecordWatchdogHeaderStall_tryResetAfterRepeats(t *testing.T) {
	syncActivity.watchdogStallTip = -1
	syncActivity.watchdogStallCount = 0
	var tryReset bool
	for i := 0; i < 3; i++ {
		_, tryReset = RecordWatchdogHeaderStall(42000, 6_000_000)
	}
	if !tryReset {
		t.Fatal("expected try-reset after 3 stalls at ancient tip with far-ahead peer")
	}
}

func TestRecordWatchdogHeaderStall_noResetWhenPeerHeightUnknown(t *testing.T) {
	syncActivity.watchdogStallTip = -1
	syncActivity.watchdogStallCount = 0
	var tryReset bool
	for i := 0; i < 3; i++ {
		_, tryReset = RecordWatchdogHeaderStall(42000, 0)
	}
	if tryReset {
		t.Fatal("unknown peer height must not trigger stuck-chain reset")
	}
}

func TestAutoRecoverSweepResetsStuckAncientMainnet(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(dir+"/headers.bin", g80[:])
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, nil, nil, nil)
	appendHeaderRange(t, j, g80[:], 1, 5000, func(h int64) uint32 {
		return 1_400_000_000 + uint32(h)*60
	})
	rewound, err := autoRecoverSweep(dir, j, nil, p, bs, nil)
	if !rewound {
		t.Fatalf("expected sweep to reset stuck ancient headers, rewound=%v err=%v", rewound, err)
	}
	tip, _ := j.TipHeight()
	if tip != 0 {
		t.Fatalf("tip after sweep=%d want 0", tip)
	}
	rewound2, _ := autoRecoverSweep(dir, j, nil, p, bs, nil)
	if rewound2 {
		t.Fatal("second sweep should be idempotent (no second genesis reset)")
	}
}

func TestMaybeResetStuckAncientInSweep_skipsLowTip(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(dir+"/h.bin", g80[:])
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, nil, nil, nil)
	reset, err := maybeResetStuckAncientInSweep(j, nil, p, bs)
	if reset || err != nil {
		t.Fatalf("tip 0: reset=%v err=%v", reset, err)
	}
}
