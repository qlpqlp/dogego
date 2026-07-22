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

func TestIsStandardTxRejectsBelowMinSize(t *testing.T) {
	tx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevIdx:  0,
			Script:   []byte{},
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{
			Value:    0,
			PkScript: []byte{0x6a, 0x00}, // OP_RETURN with empty push
		}},
	}
	raw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) >= MinStandardTxNonWitnessSize {
		t.Fatalf("fixture too large: %d bytes (want < %d)", len(raw), MinStandardTxNonWitnessSize)
	}
	err = IsStandardTx(tx, DefaultStandardPolicy(), DefaultMinRelayTxFeePerKB)
	if err == nil {
		t.Fatal("expected non-standard")
	}
	if !errors.Is(err, ErrNonStandardTx) {
		t.Fatalf("%v", err)
	}
}
