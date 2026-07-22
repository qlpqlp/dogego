// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"

	"dogego/chain"
)

// Header80 builds the 80-byte block header (no auxpow) for rebooted testnet genesis.
func Header80() ([80]byte, error) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		var z [80]byte
		return z, err
	}
	return Header80FromParams(p)
}

// BlockHashLE returns the 32-byte block id in Core uint256 memory order (double SHA256 of header80, no display reverse).
func BlockHashLE(header80 []byte) [32]byte {
	d := doubleSHA256(header80)
	var out [32]byte
	copy(out[:], d)
	return out
}

func BlockHashHex(header80 []byte) string {
	d := doubleSHA256(header80)
	for i, j := 0, len(d)-1; i < j; i, j = i+1, j-1 {
		d[i], d[j] = d[j], d[i]
	}
	return hex.EncodeToString(d)
}

// LEUint256DisplayHex renders 32 bytes (Bitcoin uint256 wire / LE layout) as lowercase explorer-style hex.
func LEUint256DisplayHex(le32 []byte) string {
	if len(le32) != 32 {
		return ""
	}
	b := append([]byte(nil), le32...)
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return hex.EncodeToString(b)
}

func doubleSHA256(b []byte) []byte {
	s := sha256.Sum256(b)
	s2 := sha256.Sum256(s[:])
	return append([]byte(nil), s2[:]...)
}

// CompactToBig converts nBits (compact) to target big.Int (Bitcoin/Dogecoin rules).
func CompactToBig(bits uint32) *big.Int {
	exponent := uint(bits >> 24)
	if exponent <= 3 {
		mantissa := bits & 0x007fffff
		return new(big.Int).SetUint64(uint64(mantissa >> (8 * (3 - exponent))))
	}
	mantissa := bits & 0x007fffff
	nn := new(big.Int).SetUint64(uint64(mantissa))
	return nn.Lsh(nn, 8*(exponent-3))
}

// CheckScryptPoW verifies Dogecoin scrypt PoW on the 80-byte header (CPureBlockHeader::GetPoWHash).
func CheckScryptPoW(header80 []byte, bits uint32) error {
	if len(header80) != 80 {
		return fmt.Errorf("header must be 80 bytes, got %d", len(header80))
	}
	powHash := scrypt102411256(header80)
	target := CompactToBig(bits)
	powInt := arithUint256LE(powHash)
	if powInt.Cmp(target) > 0 {
		return fmt.Errorf("pow hash %x exceeds target", powHash)
	}
	return nil
}

// ScryptHashLE returns the 32-byte scrypt PoW hash for an 80-byte header (Core GetPoWHash bytes).
func ScryptHashLE(header80 []byte) []byte {
	if len(header80) != 80 {
		return nil
	}
	return scrypt102411256(header80)
}

// arithUint256LE interprets 32 bytes like Bitcoin's uint256 / UintToArith256 (b[0] is LSB).
func arithUint256LE(b []byte) *big.Int {
	acc := new(big.Int)
	base := big.NewInt(1)
	bl := big.NewInt(256)
	for _, x := range b {
		acc.Add(acc, new(big.Int).Mul(big.NewInt(int64(x)), base))
		base.Mul(base, bl)
	}
	return acc
}
