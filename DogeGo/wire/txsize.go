// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

// TransactionTotalSize returns the serialized size including witness (Core GetTotalSize).
func TransactionTotalSize(tx *Tx) (int, error) {
	if tx == nil {
		return 0, nil
	}
	b, err := tx.Serialize()
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

// TransactionWeight returns BIP141 weight (base*3 + total) or 4*total for legacy txs.
func TransactionWeight(tx *Tx) (int, error) {
	if tx == nil {
		return 0, nil
	}
	total, err := TransactionTotalSize(tx)
	if err != nil {
		return 0, err
	}
	if !tx.HasWitness() {
		return total * 4, nil
	}
	base := len(tx.SerializeForHash())
	return base*3 + total, nil
}

// TransactionVirtualSize returns BIP141 vsize (weight/4 rounded up).
func TransactionVirtualSize(tx *Tx) (int, error) {
	w, err := TransactionWeight(tx)
	if err != nil {
		return 0, err
	}
	return (w + 3) / 4, nil
}
