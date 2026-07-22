// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestMaybeRewindOnBadNBitsStepsBackFurther(t *testing.T) {
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
	bs.lastBadNBitsRewind = 6480
	tip := int64(6959)
	appendFakeHeadersMonolith(t, j, g80[:], tip)
	rewound, rerr := maybeRewindOnBadNBits(j, nil, p, bs, fmt.Errorf("header batch index 0 (chain height 1 on test): bad nBits want 0x1d00ffff got 0x1d00ba8a"))
	if !rewound || rerr == nil {
		t.Fatalf("rewound=%v err=%v", rewound, rerr)
	}
	newTip, _ := j.TipHeight()
	if newTip >= 6480 {
		t.Fatalf("tip after deep rewind=%d want below 6480", newTip)
	}
}

func TestMaybeRewindOnBadNBitsRepeatBackoff(t *testing.T) {
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
	bs.lastBadNBitsRewind = 480
	targetTip := int64(959)
	fillTo := func(tip int64) {
		appendFakeHeadersMonolith(t, j, g80[:], tip)
	}
	for i := 0; i < 2; i++ {
		fillTo(targetTip)
		rewound, rerr := maybeRewindOnBadNBits(j, nil, p, bs, fmt.Errorf("bad nBits"))
		if !rewound || rerr == nil {
			t.Fatalf("attempt %d rewound=%v err=%v", i+1, rewound, rerr)
		}
	}
	fillTo(targetTip)
	rewound, rerr := maybeRewindOnBadNBits(j, nil, p, bs, fmt.Errorf("bad nBits"))
	if !rewound {
		t.Fatal("third repeated rewind should reset to genesis at low tip")
	}
	if rerr == nil || !strings.Contains(rerr.Error(), "height 0") {
		t.Fatalf("want genesis reset error, got %v", rerr)
	}
	newTip, _ := j.TipHeight()
	if newTip != 0 {
		t.Fatalf("tip=%d want 0 after repeated bad nBits reset", newTip)
	}
}

func TestMaybeRewindOnBadNBitsResetsRepeatStateAfterGenesisReset(t *testing.T) {
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
	bs.lastBadNBitsRewind = 6480
	targetTip := int64(6959)
	fillTo := func(tip int64) {
		appendFakeHeadersMonolith(t, j, g80[:], tip)
	}
	for i := 0; i < 3; i++ {
		fillTo(targetTip)
		rewound, rerr := maybeRewindOnBadNBits(j, nil, p, bs, fmt.Errorf("bad nBits"))
		if !rewound {
			t.Fatalf("attempt %d expected rewind", i+1)
		}
		if i == 2 && (rerr == nil || !strings.Contains(rerr.Error(), "height 0")) {
			t.Fatalf("third attempt expected genesis reset reason, got %v", rerr)
		}
	}
	if bs.badNBitsRepeatCount != 0 || bs.badNBitsRepeatHeight != -1 || bs.lastBadNBitsRewind != -1 {
		t.Fatalf("repeat state not reset after genesis reset: count=%d height=%d last=%d", bs.badNBitsRepeatCount, bs.badNBitsRepeatHeight, bs.lastBadNBitsRewind)
	}
	fillTo(targetTip)
	rewound, rerr := maybeRewindOnBadNBits(j, nil, p, bs, fmt.Errorf("bad nBits"))
	if !rewound {
		t.Fatal("expected rewind after reset state")
	}
	if rerr == nil || strings.Contains(rerr.Error(), "height 0") {
		t.Fatalf("expected normal rewind after reset, got %v", rerr)
	}
	if bs.badNBitsRepeatCount >= 3 {
		t.Fatalf("repeat counters should not remain stuck at reset threshold, got count=%d", bs.badNBitsRepeatCount)
	}
}

func TestMaybeRewindOnBadNBitsEachRewindMovesTipBack(t *testing.T) {
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
	// Keep tip modest: each step re-appends through targetTip (expensive on Windows).
	targetTip := int64(240)
	appendFakeHeadersMonolith(t, j, g80[:], targetTip)
	const maxSteps = 12
	for step := 0; step < maxSteps; step++ {
		tip, _ := j.TipHeight()
		if tip <= 0 {
			break
		}
		rewound, rerr := maybeRewindOnBadNBits(j, nil, p, bs, fmt.Errorf("bad nBits"))
		if !rewound {
			t.Fatalf("step %d: expected rewind at tip %d, err=%v", step, tip, rerr)
		}
		newTip, _ := j.TipHeight()
		if newTip >= tip {
			t.Fatalf("step %d: tip did not move back (%d -> %d)", step, tip, newTip)
		}
		if newTip == 0 {
			break
		}
		appendFakeHeadersMonolith(t, j, g80[:], targetTip)
	}
	if tip, _ := j.TipHeight(); tip != 0 {
		t.Fatalf("expected convergence to genesis within %d steps, tip=%d", maxSteps, tip)
	}
}

func binaryLETime(h80 []byte, t uint32) {
	h80[68] = byte(t)
	h80[69] = byte(t >> 8)
	h80[70] = byte(t >> 16)
	h80[71] = byte(t >> 24)
}

// appendFakeHeadersMonolith extends a monolith journal through tip with batched writes (Windows-friendly).
func appendFakeHeadersMonolith(t *testing.T, j *store.HeaderJournal, g80 []byte, tip int64) {
	t.Helper()
	cur, _ := j.TipHeight()
	if tip <= cur {
		return
	}
	const batch = 512
	for start := cur + 1; start <= tip; {
		end := start + batch - 1
		if end > tip {
			end = tip
		}
		buf := make([]byte, 0, (end-start+1)*80)
		for h := start; h <= end; h++ {
			h80 := make([]byte, 80)
			copy(h80, g80[:])
			binaryLETime(h80, 1_400_000_000+uint32(h)*60)
			buf = append(buf, h80...)
		}
		if err := j.AppendWireHeaderBatch(buf); err != nil {
			t.Fatal(err)
		}
		start = end + 1
	}
}

func TestBadNBitsRecoveryDecision(t *testing.T) {
	cases := []struct {
		tip, count       int64
		wantGenesis      bool
		wantPeerRotation bool
	}{
		{100, 2, false, false},
		{100, 3, true, false},
		{499_999, 3, true, false},
		{500_000, 3, false, true},
		{600_000, 5, false, true},
	}
	for _, tc := range cases {
		gen, peer := badNBitsRecoveryDecision(tc.tip, int(tc.count))
		if gen != tc.wantGenesis || peer != tc.wantPeerRotation {
			t.Fatalf("tip=%d count=%d: genesis=%v peer=%v want genesis=%v peer=%v",
				tc.tip, tc.count, gen, peer, tc.wantGenesis, tc.wantPeerRotation)
		}
	}
}
