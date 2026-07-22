// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire_test

import (
	"testing"

	"dogego/wire"
)

func TestTransactionSizeLegacy(t *testing.T) {
	tx := buildMinimalCoinbaseTx(t)
	total, err := wire.TransactionTotalSize(tx)
	if err != nil {
		t.Fatal(err)
	}
	legacy := len(tx.SerializeForHash())
	if total != legacy {
		t.Fatalf("total %d legacy %d", total, legacy)
	}
	w, err := wire.TransactionWeight(tx)
	if err != nil {
		t.Fatal(err)
	}
	if w != legacy*4 {
		t.Fatalf("weight %d want %d", w, legacy*4)
	}
	vs, err := wire.TransactionVirtualSize(tx)
	if err != nil {
		t.Fatal(err)
	}
	if vs != legacy {
		t.Fatalf("vsize %d want %d", vs, legacy)
	}
}

func TestTransactionSizeWitnessDiscount(t *testing.T) {
	tx := buildMinimalCoinbaseTx(t)
	tx.Vin[0].Witness = [][]byte{{0x01, 0x02}}
	total, err := wire.TransactionTotalSize(tx)
	if err != nil {
		t.Fatal(err)
	}
	base := len(tx.SerializeForHash())
	if total <= base {
		t.Fatalf("witness total %d should exceed base %d", total, base)
	}
	w, err := wire.TransactionWeight(tx)
	if err != nil {
		t.Fatal(err)
	}
	wantW := base*3 + total
	if w != wantW {
		t.Fatalf("weight %d want %d", w, wantW)
	}
	vs, err := wire.TransactionVirtualSize(tx)
	if err != nil {
		t.Fatal(err)
	}
	if vs != (w+3)/4 {
		t.Fatalf("vsize %d want %d", vs, (w+3)/4)
	}
	if vs >= total {
		t.Fatalf("vsize %d should be less than total %d with witness", vs, total)
	}
}
