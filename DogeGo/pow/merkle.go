// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pow

import (
	"crypto/sha256"
	"fmt"
	"math/big"
)

// CheckMerkleBranch walks a merkle path (Core CAuxPow::CheckMerkleBranch).
func CheckMerkleBranch(hash [32]byte, branch [][32]byte, index int32) [32]byte {
	if index == -1 {
		return [32]byte{}
	}
	cur := hash
	i := index
	for _, sib := range branch {
		var left, right []byte
		if i&1 != 0 {
			left = sib[:]
			right = cur[:]
		} else {
			left = cur[:]
			right = sib[:]
		}
		sum := append(append([]byte(nil), left...), right...)
		h := sha256.Sum256(sum)
		h2 := sha256.Sum256(h[:])
		cur = h2
		i >>= 1
	}
	return cur
}

// AuxExpectedIndex is Core CAuxPow::getExpectedIndex (unsigned wrap).
func AuxExpectedIndex(nNonce uint32, chainID int32, merkleHeight uint) int {
	r := nNonce
	r = r*1103515245 + 12345
	r += uint32(chainID)
	r = r*1103515245 + 12345
	mod := uint32(1) << merkleHeight
	return int(r % mod)
}

// TargetFromCompact decodes nBits like arith_uint256::SetCompact (rejects invalid encodings).
func TargetFromCompact(bits uint32) (*big.Int, error) {
	exponent := bits >> 24
	mantissa := bits & 0x007fffff
	if mantissa != 0 && exponent <= 3 {
		if mantissa%(1<<(8*(3-exponent))) != 0 {
			return nil, fmt.Errorf("non-zero mantissa with exponent<=3")
		}
	}
	if mantissa != 0 && ((exponent > 34) ||
		(mantissa > 0xff && exponent > 33) ||
		(mantissa > 0xffff && exponent > 32)) {
		return nil, fmt.Errorf("compact overflow")
	}
	if mantissa != 0 && (bits&0x00800000) != 0 {
		return nil, fmt.Errorf("negative compact")
	}
	var bn *big.Int
	if exponent <= 3 {
		bn = new(big.Int).SetUint64(uint64(mantissa >> (8 * (3 - exponent))))
	} else {
		bn = new(big.Int).SetUint64(uint64(mantissa))
		bn.Lsh(bn, uint(8*(exponent-3)))
	}
	return bn, nil
}

// CheckProofOfWorkLE checks scrypt hash (LE uint256 layout) against nBits and pow limit (MSB-first hex integer).
func CheckProofOfWorkLE(hashLE []byte, bits uint32, powLimitMSBHex string) error {
	if len(hashLE) != 32 {
		return fmt.Errorf("pow hash len %d", len(hashLE))
	}
	target, err := TargetFromCompact(bits)
	if err != nil {
		return err
	}
	limit := new(big.Int)
	if _, ok := limit.SetString(powLimitMSBHex, 16); !ok {
		return fmt.Errorf("bad pow limit hex")
	}
	if target.Cmp(limit) > 0 {
		return fmt.Errorf("target above pow limit")
	}
	hInt := arithUint256LE(hashLE)
	if hInt.Cmp(target) > 0 {
		return fmt.Errorf("insufficient work")
	}
	return nil
}

// DifficultyFromCompact returns network difficulty as pow_limit / target (same scaling as Core getdifficulty for scrypt chains).
func DifficultyFromCompact(bits uint32) (float64, error) {
	target, err := TargetFromCompact(bits)
	if err != nil {
		return 0, err
	}
	if target.Sign() == 0 {
		return 0, fmt.Errorf("zero target")
	}
	limit, ok := new(big.Int).SetString(dogePowLimitHex, 16)
	if !ok {
		return 0, fmt.Errorf("pow limit")
	}
	r := new(big.Rat).SetFrac(limit, target)
	f, _ := r.Float64()
	return f, nil
}
