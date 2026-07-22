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

func TestTxRowEncodeDecodeFee(t *testing.T) {
	row := TxRow{
		TxID: "abc", Category: "send", Address: "Daddr",
		AmountKoinu: -100, FeeKoinu: 1_000_000, Vout: 0, BlockHeight: 42,
	}
	key := txKey(row)
	got, ok := decodeTxRow(key, row.encodeValue())
	if !ok {
		t.Fatal("decode failed")
	}
	if got.FeeKoinu != row.FeeKoinu {
		t.Fatalf("fee %d want %d", got.FeeKoinu, row.FeeKoinu)
	}
}

func TestTxDBImportAndCursor(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "wallet.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := []TxRow{
		{TxID: "aa", Category: "receive", Address: "addr1", AmountKoinu: 100, Vout: 0, BlockHeight: 10},
		{TxID: "bb", Category: "send", Address: "addr1", AmountKoinu: -50, Vout: 1, BlockHeight: 12},
	}
	if err := db.ImportLegacy(rows); err != nil {
		t.Fatal(err)
	}
	max, err := db.MaxScannedHeight()
	if err != nil || max != 12 {
		t.Fatalf("max=%d err=%v want 12", max, err)
	}
	cur, err := db.ScanCursor()
	if err != nil || cur != 12 {
		t.Fatalf("cursor=%d err=%v want 12", cur, err)
	}
	list, err := db.ListTx()
	if err != nil || len(list) != 2 {
		t.Fatalf("list len=%d err=%v", len(list), err)
	}

	if err := db.ReplaceFromHeight(12, []TxRow{
		{TxID: "cc", Category: "receive", Address: "addr2", AmountKoinu: 5, Vout: 0, BlockHeight: 12},
	}); err != nil {
		t.Fatal(err)
	}
	list, _ = db.ListTx()
	if len(list) != 2 {
		t.Fatalf("after replace len=%d want 2 (10 + new 12)", len(list))
	}
}

func TestTxDBAppendBlock(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "wallet.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.AppendBlock(5, []TxRow{
		{TxID: "x", Category: "receive", Address: "a", AmountKoinu: 1, Vout: 0, BlockHeight: 5},
	}); err != nil {
		t.Fatal(err)
	}
	cur, _ := db.ScanCursor()
	if cur != 5 {
		t.Fatalf("cursor=%d want 5", cur)
	}
}
