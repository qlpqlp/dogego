// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"testing"

	"dogego/wire"
)

func TestAreInputsStandardRejectsHeavyP2SHRedeem(t *testing.T) {
	redeem := make([]byte, 0, 32)
	for i := 0; i < 16; i++ {
		redeem = append(redeem, 0xac) // OP_CHECKSIG
	}
	p2sh := p2shFromRedeem(redeem)
	view := stubPrevOutView{outpointKey([32]byte{1}, 0): {Value: 1e8, PkScript: p2sh}}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff, Script: append([]byte{byte(len(redeem))}, redeem...)}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: p2pkhScript()}},
	}
	err := AreInputsStandard(tx, view)
	if !errors.Is(err, ErrNonStandardTx) {
		t.Fatalf("got %v", err)
	}
}

func TestAreInputsStandardRejectsNonStandardPrevout(t *testing.T) {
	view := stubPrevOutView{outpointKey([32]byte{2}, 0): {Value: 1e8, PkScript: []byte{0x99, 0x88}}}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{2}, PrevIdx: 0, Sequence: 0xffffffff, Script: []byte{0x51}}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: p2pkhScript()}},
	}
	err := AreInputsStandard(tx, view)
	if !errors.Is(err, ErrNonStandardTx) {
		t.Fatalf("got %v", err)
	}
}

func p2shFromRedeem(redeem []byte) []byte {
	h := hash160(redeem)
	return []byte{0xa9, 0x14, h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7], h[8], h[9], h[10], h[11], h[12], h[13], h[14], h[15], h[16], h[17], h[18], h[19], 0x87}
}
