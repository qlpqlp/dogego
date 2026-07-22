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

func TestChangeKeypoolOnCreate(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if got := w.ChangeKeypoolSize(); got < keypoolRefillThreshold {
		t.Fatalf("change keypool %d want >= %d", got, keypoolRefillThreshold)
	}
}

func TestChangeKeypoolAfterGetRawChange(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	w.hdChangeKeypool = make([]uint32, keypoolRefillThreshold-1)
	for i := range w.hdChangeKeypool {
		w.hdChangeKeypool[i] = w.hdChangeNext
		w.hdChangeNext++
	}
	w.mu.Unlock()
	if _, err := w.NewChangeAddress(); err != nil {
		t.Fatal(err)
	}
	if got := w.ChangeKeypoolSize(); got < keypoolRefillThreshold {
		t.Fatalf("after issue change keypool %d want >= %d", got, keypoolRefillThreshold)
	}
}

func TestChangeIsKeypoolPeekAndCommit(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	peek := w.PeekChangeAddress()
	if peek == "" {
		t.Fatal("empty peek")
	}
	if !w.IsChangeInKeypool(peek) {
		t.Fatal("peeked change should be iskeypool")
	}
	listRows := w.ListAddressEntries(p.PubkeyHashAddrID, p.ScriptHashAddrID)
	var found bool
	for _, e := range listRows {
		if e.Address != peek {
			continue
		}
		found = true
		if !e.IsChange || !e.IsKeypool {
			t.Fatalf("list entry %+v", e)
		}
	}
	if !found {
		t.Fatal("peeked change missing from list")
	}
	if err := w.CommitChangeAddress(peek); err != nil {
		t.Fatal(err)
	}
	if w.IsChangeInKeypool(peek) {
		t.Fatal("committed change must leave keypool")
	}
}
