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

func TestCheckAbsurdTxFee(t *testing.T) {
	inVal := DefaultMaxAbsurdTxFeeKoinu + 1_000_000
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	view := stubFeeView{outpointKey([32]byte{1}, 0): {Value: inVal, PkScript: []byte{0x51}}}
	err := CheckAbsurdTxFee(tx, view, DefaultMaxAbsurdTxFeeKoinu)
	if !errors.Is(err, ErrAbsurdlyHighFee) {
		t.Fatalf("got %v", err)
	}
}
