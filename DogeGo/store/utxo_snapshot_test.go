// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"os"
	"path/filepath"
	"testing"

	"dogego/wire"
)

func TestUtxoSnapshotRoundTrip(t *testing.T) {
	u := NewUtxoCache()
	gen := wire.ParsedBlock{Txs: []*wire.Tx{{Version: 1, Vin: []wire.TxIn{{}}, Vout: []wire.TxOut{{Value: 50e8, PkScript: []byte{0xaa}}}}}}
	if err := u.ApplyBlock(&gen, 0); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "utxo.cache")
	if err := u.SaveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadUtxoSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("nil loaded")
	}
	if loaded.TipHeight() != 0 || loaded.Count() != 1 {
		t.Fatalf("tip=%d count=%d", loaded.TipHeight(), loaded.Count())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
