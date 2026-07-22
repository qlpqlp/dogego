// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pow

import (
	"math/big"
)

// BlockProofFromBits returns per-block chain work for nBits (Bitcoin Core GetBitsProof).
// See https://github.com/bitcoin/bitcoin/blob/master/src/chain.cpp (GetBitsProof).
func BlockProofFromBits(bits uint32) (*big.Int, error) {
	target, err := TargetFromCompact(bits)
	if err != nil {
		return nil, err
	}
	if target.Sign() == 0 {
		return big.NewInt(0), nil
	}
	// max uint256 = 2^256 - 1 ; ~target in Core is bitwise NOT on 256 bits.
	maxU := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	tp1 := new(big.Int).Add(target, big.NewInt(1))
	notT := new(big.Int).Sub(maxU, target)
	w := new(big.Int).Div(notT, tp1)
	return w.Add(w, big.NewInt(1)), nil
}

// ChainworkHex renders cumulative chain work as lowercase hex (Core JSON "chainwork").
func ChainworkHex(sum *big.Int) string {
	if sum == nil || sum.Sign() == 0 {
		return "0"
	}
	return sum.Text(16)
}
