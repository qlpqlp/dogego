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

func TestCheckTransactionRejectsUnspendableWithValue(t *testing.T) {
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout: []wire.TxOut{{
			Value:    1000,
			PkScript: []byte{0x6a, 0x04, 0xde, 0xad, 0xbe, 0xef},
		}},
	}
	if err := CheckTransaction(tx, false); err == nil {
		t.Fatal("expected unspendable output error")
	}
}
