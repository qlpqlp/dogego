// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"path/filepath"
	"testing"
)

func TestNodeTipSeparateFromReceive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	w, err := LoadOrCreate(path, 0x41)
	if err != nil {
		t.Fatal(err)
	}
	recv, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	tip, err := w.EnableNodeTip()
	if err != nil {
		t.Fatal(err)
	}
	if tip == recv {
		t.Fatalf("tip %q must differ from receive %q", tip, recv)
	}
	if !w.ContainsAddress(tip) {
		t.Fatal("tip not tracked")
	}
	w2, err := LoadOrCreate(path, 0x41)
	if err != nil {
		t.Fatal(err)
	}
	if !w2.NodeTipEnabled() || w2.NodeTipAddress() != tip {
		t.Fatalf("reload tip enabled=%v addr=%q", w2.NodeTipEnabled(), w2.NodeTipAddress())
	}
	scripts := w2.SpendScripts()
	if len(scripts) < 2 {
		t.Fatalf("expected receive+tip scripts, got %d", len(scripts))
	}
}
