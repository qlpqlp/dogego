// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"testing"

	"dogego/mempool"
	"dogego/wire"
)

func TestCheckSpendConflictsReturnsRBFInsufficientFee(t *testing.T) {
	raw, prep, view, err := BuildRBFInsufficientFeeDifferentialFixture()
	if err != nil {
		t.Fatal(err)
	}
	mp := mempool.New(50)
	if err := prep(mp); err != nil {
		t.Fatal(err)
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	adm := MempoolAdmission{
		View:    view,
		Pool:    mp,
		RBFPool: mp,
	}
	err = adm.CheckSpendConflicts(tx)
	if !errors.Is(err, ErrRBFInsufficientFee) {
		t.Fatalf("got %v (reason %q)", err, MempoolRejectReason(err))
	}
}
