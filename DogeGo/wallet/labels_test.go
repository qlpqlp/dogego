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

func TestWalletLabelsRoundTrip(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	addr := w.Address()
	if err := w.SetLabel(addr, "receive"); err != nil {
		t.Fatal(err)
	}
	w2, err := LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if w2.Label(addr) != "receive" {
		t.Fatalf("label %q", w2.Label(addr))
	}
	labels := w2.ListLabels()
	if len(labels) != 1 || labels[0] != "receive" {
		t.Fatalf("list %#v", labels)
	}
	if err := w2.SetLabel(addr, ""); err != nil {
		t.Fatal(err)
	}
	if w2.Label(addr) != "" {
		t.Fatal("expected cleared label")
	}
}
