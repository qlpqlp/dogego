// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pow

import (
	"math/big"
	"sync"
)

const dogePowLimitHex = "00000fffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

var (
	powLimitCompactOnce sync.Once
	powLimitCompactVal  uint32
)

// DogePowLimitCompact returns Core's UintToArith256(powLimit).GetCompact() for Dogecoin main/test.
func DogePowLimitCompact() uint32 {
	powLimitCompactOnce.Do(func() {
		limit, _ := new(big.Int).SetString(dogePowLimitHex, 16)
		powLimitCompactVal = bigIntToCompact(limit)
	})
	return powLimitCompactVal
}

// CompactFromBigInt encodes a positive target as nBits (Core arith_uint256::GetCompact).
func CompactFromBigInt(n *big.Int) uint32 {
	return bigIntToCompact(n)
}

func bigIntToCompact(n *big.Int) uint32 {
	if n == nil || n.Sign() == 0 {
		return 0
	}
	nSize := (n.BitLen() + 7) / 8
	mant := new(big.Int).Set(n)
	var nCompact uint32
	if nSize <= 3 {
		mant.Lsh(mant, uint(8*(3-nSize)))
		nCompact = uint32(mant.Uint64())
	} else {
		mant.Rsh(mant, uint(8*(nSize-3)))
		nCompact = uint32(mant.Uint64() & 0xffffff)
	}
	if nCompact&0x00800000 != 0 {
		nCompact >>= 8
		nSize++
	}
	if nSize > 255 {
		return 0
	}
	nCompact |= uint32(nSize) << 24
	return nCompact
}
