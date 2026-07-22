// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

// MaxBIP125RBFSequence is Core MAX_BIP125_RBF_SEQUENCE (inclusive opt-in signal).
const MaxBIP125RBFSequence = 0xfffffffd

// IsBIP125Replaceable reports whether any input signals BIP125 replaceability (nSequence <= 0xfffffffd).
func IsBIP125Replaceable(tx *Tx) bool {
	if tx == nil {
		return false
	}
	for i := range tx.Vin {
		if tx.Vin[i].Sequence <= MaxBIP125RBFSequence {
			return true
		}
	}
	return false
}
