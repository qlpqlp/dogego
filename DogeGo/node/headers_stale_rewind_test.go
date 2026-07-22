// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func TestMaybeRewindStaleHeaderTimes(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	// Build a short chain with compressed 60s spacing (simulates stalled partial sync).
	prevHash := pow.BlockHashLE(g80[:])
	prev := append([]byte(nil), g80[:]...)
	for i := 1; i <= 100; i++ {
		h := append([]byte(nil), prev...)
		copy(h[4:36], prevHash[:])
		h[76] ^= byte(i)
		binary.LittleEndian.PutUint32(h[68:72], 1_000_000+uint32(i*60))
		binary.LittleEndian.PutUint32(h[72:76], 0x1c1f6a87)
		if err := j.AppendHeaders([][]byte{h}); err != nil {
			t.Fatal(err)
		}
		prevHash = pow.BlockHashLE(h)
		prev = h
	}
	tip, _ := j.TipHeight()
	tip80, _ := j.ReadHeaderAt(tip)
	tipTime := binary.LittleEndian.Uint32(tip80[68:72])
	peer := append([]byte(nil), prev...)
	copy(peer[4:36], prevHash[:])
	peer[76] ^= 0x55
	binary.LittleEndian.PutUint32(peer[68:72], tipTime+200_000) // far ahead of local tip
	rewound, err := maybeRewindStaleHeaderTimes(j, nil, p, []wire.DecodedHeader{{Header80: peer}}, nil)
	if !rewound || err == nil {
		t.Fatalf("rewound=%v err=%v want rewind", rewound, err)
	}
	if !strings.Contains(err.Error(), "rewound journal") {
		t.Fatalf("err: %v", err)
	}
	newTip, _ := j.TipHeight()
	if newTip != 0 {
		t.Fatalf("after rewind tip=%d want 0 (100 is below first 240 retarget)", newTip)
	}
}

func TestRecoverableHeaderPeerErr_rewoundJournal(t *testing.T) {
	if recoverableHeaderPeerErr(errString("headers: rewound journal to height 0 (retry getheaders)")) {
		t.Fatal("rewound journal retries on same peer, not peer rotation")
	}
}

func TestMaybeRewindCompressedHeaderPeriod(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	prevHash := pow.BlockHashLE(g80[:])
	prev := append([]byte(nil), g80[:]...)
	baseTime := binary.LittleEndian.Uint32(g80[68:72]) + 10_000
	for i := 1; i <= 479; i++ {
		h := append([]byte(nil), prev...)
		copy(h[4:36], prevHash[:])
		h[76] ^= byte(i)
		binary.LittleEndian.PutUint32(h[68:72], baseTime+uint32(i*60))
		binary.LittleEndian.PutUint32(h[72:76], 0x1c1f6a87)
		if err := j.AppendHeaders([][]byte{h}); err != nil {
			t.Fatal(err)
		}
		prevHash = pow.BlockHashLE(h)
		prev = h
	}
	rewound, err := maybeRewindCompressedHeaderPeriod(j, nil, p, nil)
	if !rewound || err == nil {
		t.Fatalf("rewound=%v err=%v want compressed-period rewind", rewound, err)
	}
	newTip, _ := j.TipHeight()
	if newTip != 240 {
		t.Fatalf("after rewind tip=%d want 240 (previous difficulty period)", newTip)
	}
}

func TestMaybeRewindCompressedHeaderPeriod_partialPeriodNoRewind(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	prevHash := pow.BlockHashLE(g80[:])
	prev := append([]byte(nil), g80[:]...)
	baseTime := binary.LittleEndian.Uint32(g80[68:72]) + 10_000
	// Fewer than minBlocksForPeriodSpanCheck blocks: not enough data to judge compression.
	for i := 1; i <= 20; i++ {
		h := append([]byte(nil), prev...)
		copy(h[4:36], prevHash[:])
		h[76] ^= byte(i)
		binary.LittleEndian.PutUint32(h[68:72], baseTime+uint32(i*60))
		binary.LittleEndian.PutUint32(h[72:76], 0x1c1f6a87)
		if err := j.AppendHeaders([][]byte{h}); err != nil {
			t.Fatal(err)
		}
		prevHash = pow.BlockHashLE(h)
		prev = h
	}
	rewound, err := maybeRewindCompressedHeaderPeriod(j, nil, p, nil)
	if rewound || err != nil {
		t.Fatalf("short partial period: rewound=%v err=%v want no rewind", rewound, err)
	}
}

// TestMaybeRewindCompressedHeaderPeriod_midPeriodMainnet4080 catches fast-sync compressed
// timestamps mid retarget window (mainnet bad nBits loop at tip ~4000 before 4080).
func TestMaybeRewindCompressedHeaderPeriod_midPeriodMainnet4080(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	prevHash := pow.BlockHashLE(g80[:])
	prev := append([]byte(nil), g80[:]...)
	baseTime := binary.LittleEndian.Uint32(g80[68:72]) + 10_000
	// Same geometry as mainnet 3840..4000 but at heights 240..400 (240-block retarget).
	for i := 1; i <= 400; i++ {
		h := append([]byte(nil), prev...)
		copy(h[4:36], prevHash[:])
		h[76] ^= byte(i & 0xff)
		spacing := uint32(120)
		if i >= 240 {
			spacing = 60 // compressed fast-sync burst in active period
		}
		binary.LittleEndian.PutUint32(h[68:72], baseTime+uint32(i)*spacing)
		binary.LittleEndian.PutUint32(h[72:76], 0x1c1f6a87)
		if err := j.AppendHeaders([][]byte{h}); err != nil {
			t.Fatal(err)
		}
		prevHash = pow.BlockHashLE(h)
		prev = h
	}
	rewound, err := maybeRewindCompressedHeaderPeriod(j, nil, p, nil)
	if !rewound || err == nil {
		t.Fatalf("mid-period compressed: rewound=%v err=%v want rewind before next retarget", rewound, err)
	}
	newTip, _ := j.TipHeight()
	if newTip != 240 {
		t.Fatalf("after rewind tip=%d want 240 (previous difficulty period)", newTip)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
