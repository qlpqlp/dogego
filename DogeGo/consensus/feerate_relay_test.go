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

func TestMeetsPeerFeeFilter(t *testing.T) {
	if !MeetsPeerFeeFilter(100_000, 0) {
		t.Fatal("zero filter")
	}
	if !MeetsPeerFeeFilter(100_000, 100_000) {
		t.Fatal("equal")
	}
	if MeetsPeerFeeFilter(99_999, 100_000) {
		t.Fatal("below filter")
	}
}

func TestTxFeeRateKoinuPerKB(t *testing.T) {
	tx := &wire.Tx{
		Vin:  []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0}},
		Vout: []wire.TxOut{{Value: 900_000, PkScript: []byte{0x76, 0xa9, 0x14}}},
	}
	view := stubPrevOutView{outpointKey([32]byte{1}, 0): {Value: 1_000_000}}
	rate, ok := TxFeeRateKoinuPerKB(tx, nil, view)
	if !ok || rate == 0 {
		t.Fatalf("rate %d ok %v", rate, ok)
	}
}
