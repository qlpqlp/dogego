// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import "testing"

func TestIsBIP125Replaceable(t *testing.T) {
	tx := &Tx{
		Version: 1,
		Vin:     []TxIn{{Sequence: 0xffffffff}},
		Vout:    []TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	if IsBIP125Replaceable(tx) {
		t.Fatal("final sequence")
	}
	tx.Vin[0].Sequence = MaxBIP125RBFSequence
	if !IsBIP125Replaceable(tx) {
		t.Fatal("opt-in")
	}
	tx.Vin[0].Sequence = 0
	if !IsBIP125Replaceable(tx) {
		t.Fatal("zero sequence")
	}
}
