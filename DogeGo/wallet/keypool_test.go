// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"path/filepath"
	"testing"

	"dogego/chain"
)

func TestEnsureKeypoolOnLoad(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	path := filepath.Join(t.TempDir(), "wallet.json")
	w, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	w.hdKeypool = []uint32{100, 101, 102, 103}
	w.hdExternalNext = 104
	w.mu.Unlock()
	if err := w.EnsureKeypoolOnLoad(); err != nil {
		t.Fatal(err)
	}
	if got := w.KeypoolSize(); got < keypoolRefillThreshold {
		t.Fatalf("keypool %d want >= %d", got, keypoolRefillThreshold)
	}
}

func TestEnsureKeypoolAfterNewAddress(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	w.hdKeypool = make([]uint32, keypoolRefillThreshold-1)
	for i := range w.hdKeypool {
		w.hdKeypool[i] = w.hdExternalNext
		w.hdExternalNext++
	}
	w.mu.Unlock()
	if _, err := w.NewReceiveAddress(); err != nil {
		t.Fatal(err)
	}
	if got := w.KeypoolSize(); got < keypoolRefillThreshold {
		t.Fatalf("after issue keypool %d want >= %d", got, keypoolRefillThreshold)
	}
}

func TestKeypoolRefillFillsToTarget(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	w.hdKeypool = w.hdKeypool[:99]
	w.hdChangeKeypool = w.hdChangeKeypool[:99]
	w.mu.Unlock()
	if err := w.KeypoolRefill(100); err != nil {
		t.Fatal(err)
	}
	if got := w.KeypoolSize(); got != 100 {
		t.Fatalf("receive keypool %d want 100", got)
	}
	if got := w.ChangeKeypoolSize(); got != 100 {
		t.Fatalf("change keypool %d want 100", got)
	}
	if err := w.KeypoolRefill(100); err != nil {
		t.Fatal(err)
	}
	if got := w.KeypoolSize(); got != 100 {
		t.Fatalf("second refill receive %d want 100", got)
	}
	if got := w.ChangeKeypoolSize(); got != 100 {
		t.Fatalf("second refill change %d want 100", got)
	}
}

func TestKeypoolRefillEmptyToCustomSize(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	w.hdKeypool = nil
	w.hdChangeKeypool = nil
	w.syncReceiveNextFromPoolLocked()
	w.syncChangeNextFromPoolLocked()
	w.mu.Unlock()
	if err := w.KeypoolRefill(50); err != nil {
		t.Fatal(err)
	}
	if got := w.KeypoolSize(); got != 50 {
		t.Fatalf("receive keypool %d want 50", got)
	}
	if got := w.ChangeKeypoolSize(); got != 50 {
		t.Fatalf("change keypool %d want 50", got)
	}
}

func TestInitHDExternalNextPastKeypool(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	next := w.hdExternalNext
	maxPool := uint32(0)
	for _, idx := range w.hdKeypool {
		if idx > maxPool {
			maxPool = idx
		}
	}
	w.mu.Unlock()
	if next != defaultKeypoolSize {
		t.Fatalf("hdExternalNext=%d want %d", next, defaultKeypoolSize)
	}
	if maxPool+1 != next {
		t.Fatalf("max pool %d next %d", maxPool, next)
	}
}
