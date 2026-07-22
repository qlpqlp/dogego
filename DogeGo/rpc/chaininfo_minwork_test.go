// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/binary"
	"path/filepath"
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestIbdProgressBelowMinimumChainWork(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(t.TempDir(), "h.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	// Single genesis header - work far below mainnet minimum.
	ibd, _ := ibdProgress(j, "main", 0, -1, 0, chain.DefaultMaxTipAge, time.Now().Unix(), nil)
	if !ibd {
		t.Fatal("expected IBD when header chain work below minimum")
	}
}

func TestIbdProgressHeadersAheadOfChainActive(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(t.TempDir(), "h.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	ibd, prog := ibdProgress(j, "test", 1000, 500, 2, chain.DefaultMaxTipAge, time.Now().Unix(), nil)
	if !ibd {
		t.Fatal("expected IBD when headers ahead of chainActive")
	}
	want := float64(3) / float64(1001)
	if prog < want-0.001 || prog > want+0.001 {
		t.Fatalf("progress %v want ~%v", prog, want)
	}
}

func TestIbdProgressBodiesLag(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(t.TempDir(), "h.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	ibd, prog := ibdProgress(j, "test", 5, 2, 2, chain.DefaultMaxTipAge, time.Now().Unix(), nil)
	if !ibd {
		t.Fatal("expected IBD when bodies lag headers")
	}
	if prog >= 1 {
		t.Fatalf("progress %v", prog)
	}
}

func TestIbdProgressStaleTip(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Unix() - chain.DefaultMaxTipAge - 3600
	binary.LittleEndian.PutUint32(g80[68:72], uint32(stale))
	j, err := store.OpenHeaderJournal(filepath.Join(t.TempDir(), "h.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	ibd, _ := ibdProgress(j, "test", 0, 0, 0, chain.DefaultMaxTipAge, now, nil)
	if !ibd {
		t.Fatal("expected IBD when chainActive tip time exceeds -maxtipage")
	}
	ibdFresh, _ := ibdProgress(j, "test", 0, 0, 0, chain.DefaultMaxTipAge, stale+chain.DefaultMaxTipAge/2, nil)
	if ibdFresh {
		t.Fatal("expected caught up when adjusted now is near stale tip time")
	}
}

func TestChainTipAheadStatus(t *testing.T) {
	if s := chainTipAheadStatus(5, 10, 8, true); s != "valid-headers" {
		t.Fatalf("got %q want valid-headers", s)
	}
	if s := chainTipAheadStatus(5, 10, -1, true); s != "headers-only" {
		t.Fatalf("got %q want headers-only", s)
	}
	if s := chainTipAheadStatus(10, 10, 10, true); s != "" {
		t.Fatalf("got %q want empty", s)
	}
}
