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

func TestRemovePrunedImportByTxID(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	txid := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	script := []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x88, 0xac}
	if err := w.ImportPrunedReceive(txid, 1, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 0, 1e8, script); err != nil {
		t.Fatal(err)
	}
	if !w.RemovePrunedImportByTxID(txid) {
		t.Fatal("expected remove")
	}
	if len(w.ListPrunedImports()) != 0 {
		t.Fatalf("imports left: %d", len(w.ListPrunedImports()))
	}
	if w.RemovePrunedImportByTxID(txid) {
		t.Fatal("expected false on second remove")
	}
}
