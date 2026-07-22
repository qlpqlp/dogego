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

func TestLockedOutpointsPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	w, err := LoadOrCreate(path, 0x1e)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.SetLockedOutpoints(false, []LockedOutpoint{{TxID: "ab" + strings64(), Vout: 1}}); err != nil {
		t.Fatal(err)
	}
	w2, err := LoadOrCreate(path, 0x1e)
	if err != nil {
		t.Fatal(err)
	}
	if !w2.IsLockedOutpoint("ab"+strings64(), 1) {
		t.Fatal("not locked after reload")
	}
	if err := w2.SetLockedOutpoints(true, []LockedOutpoint{{TxID: "ab" + strings64(), Vout: 1}}); err != nil {
		t.Fatal(err)
	}
	if w2.IsLockedOutpoint("ab"+strings64(), 1) {
		t.Fatal("still locked")
	}
}

func strings64() string {
	const h = "0000000000000000000000000000000000000000000000000000000000000001"
	return h
}
