// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package txdb

import (
	"path/filepath"
	"testing"
)

func TestTxHexPutGet(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "wallet.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	txid := "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234"
	hexStr := "0100000001deadbeef"
	if err := db.PutTxHex(txid, hexStr); err != nil {
		t.Fatal(err)
	}
	got, ok := db.GetTxHex(txid)
	if !ok || got != hexStr {
		t.Fatalf("get ok=%v hex=%q want %q", ok, got, hexStr)
	}
}
