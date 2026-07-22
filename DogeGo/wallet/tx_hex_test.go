// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"testing"

	"dogego/wallet/txdb"
)

func TestRememberLookupTxHex(t *testing.T) {
	dir := t.TempDir()
	w := &Disk{path: dir}
	if err := w.withTxDB(func(db *txdb.DB) error { return nil }); err != nil {
		t.Fatal(err)
	}
	txid := "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234"
	hexStr := "0100000001cafe"
	if err := w.RememberTxHex(txid, hexStr); err != nil {
		t.Fatal(err)
	}
	got, ok := w.LookupTxHex(txid)
	if !ok || got != hexStr {
		t.Fatalf("lookup ok=%v hex=%q", ok, got)
	}
}
