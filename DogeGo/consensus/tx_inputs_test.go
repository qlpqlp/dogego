// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/wire"
)

func TestCheckTxInputsRejectsBelowOut(t *testing.T) {
	view := stubPrevOutView{outpointKey([32]byte{1}, 0): {Value: 1000, PkScript: p2pkhScript()}}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2000, PkScript: p2pkhScript()}},
	}
	err := CheckTxInputs(tx, view)
	if err == nil || err.Error() != "bad-txns-in-belowout" {
		t.Fatalf("got %v", err)
	}
}

func TestCheckTxInputsMissingPrevout(t *testing.T) {
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{2}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: p2pkhScript()}},
	}
	err := CheckTxInputs(tx, stubPrevOutView{})
	if err == nil {
		t.Fatal("expected error")
	}
}
