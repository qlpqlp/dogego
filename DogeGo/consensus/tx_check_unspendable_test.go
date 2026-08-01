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

func TestCheckTransactionAllowsUnspendableWithValue(t *testing.T) {
	// Dogecoin Core CheckTransaction accepts OP_RETURN with value (burn). DogeGo must
	// match so mainnet blocks such as height 470683 connect.
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout: []wire.TxOut{{
			Value:    1000,
			PkScript: []byte{0x6a, 0x04, 0xde, 0xad, 0xbe, 0xef},
		}},
	}
	if err := CheckTransaction(tx, false); err != nil {
		t.Fatalf("Core allows unspendable-with-value at consensus: %v", err)
	}
}
