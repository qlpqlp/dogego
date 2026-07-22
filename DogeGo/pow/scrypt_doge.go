// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Dogecoin/Litecoin scrypt_1024_1_1_256 (src/crypto/scrypt.cpp), pure Go.
package pow

import (
	"crypto/sha256"
	"encoding/binary"

	"golang.org/x/crypto/pbkdf2"
)

const scryptScratchWords = 1024 * 32 // 1024 rows of X[32] uint32

func rotl32(a uint32, b uint32) uint32 {
	return (a << b) | (a >> (32 - b))
}

func xorSalsa8(B *[16]uint32, Bx *[16]uint32) {
	x00 := (B[0] ^ Bx[0])
	B[0] = x00
	x01 := (B[1] ^ Bx[1])
	B[1] = x01
	x02 := (B[2] ^ Bx[2])
	B[2] = x02
	x03 := (B[3] ^ Bx[3])
	B[3] = x03
	x04 := (B[4] ^ Bx[4])
	B[4] = x04
	x05 := (B[5] ^ Bx[5])
	B[5] = x05
	x06 := (B[6] ^ Bx[6])
	B[6] = x06
	x07 := (B[7] ^ Bx[7])
	B[7] = x07
	x08 := (B[8] ^ Bx[8])
	B[8] = x08
	x09 := (B[9] ^ Bx[9])
	B[9] = x09
	x10 := (B[10] ^ Bx[10])
	B[10] = x10
	x11 := (B[11] ^ Bx[11])
	B[11] = x11
	x12 := (B[12] ^ Bx[12])
	B[12] = x12
	x13 := (B[13] ^ Bx[13])
	B[13] = x13
	x14 := (B[14] ^ Bx[14])
	B[14] = x14
	x15 := (B[15] ^ Bx[15])
	B[15] = x15

	for i := 0; i < 8; i += 2 {
		x04 ^= rotl32(x00+x12, 7)
		x09 ^= rotl32(x05+x01, 7)
		x14 ^= rotl32(x10+x06, 7)
		x03 ^= rotl32(x15+x11, 7)

		x08 ^= rotl32(x04+x00, 9)
		x13 ^= rotl32(x09+x05, 9)
		x02 ^= rotl32(x14+x10, 9)
		x07 ^= rotl32(x03+x15, 9)

		x12 ^= rotl32(x08+x04, 13)
		x01 ^= rotl32(x13+x09, 13)
		x06 ^= rotl32(x02+x14, 13)
		x11 ^= rotl32(x07+x03, 13)

		x00 ^= rotl32(x12+x08, 18)
		x05 ^= rotl32(x01+x13, 18)
		x10 ^= rotl32(x06+x02, 18)
		x15 ^= rotl32(x11+x07, 18)

		x01 ^= rotl32(x00+x03, 7)
		x06 ^= rotl32(x05+x04, 7)
		x11 ^= rotl32(x10+x09, 7)
		x12 ^= rotl32(x15+x14, 7)

		x02 ^= rotl32(x01+x00, 9)
		x07 ^= rotl32(x06+x05, 9)
		x08 ^= rotl32(x11+x10, 9)
		x13 ^= rotl32(x12+x15, 9)

		x03 ^= rotl32(x02+x01, 13)
		x04 ^= rotl32(x07+x06, 13)
		x09 ^= rotl32(x08+x11, 13)
		x14 ^= rotl32(x13+x12, 13)

		x00 ^= rotl32(x03+x02, 18)
		x05 ^= rotl32(x04+x07, 18)
		x10 ^= rotl32(x09+x08, 18)
		x15 ^= rotl32(x14+x13, 18)
	}
	B[0] += x00
	B[1] += x01
	B[2] += x02
	B[3] += x03
	B[4] += x04
	B[5] += x05
	B[6] += x06
	B[7] += x07
	B[8] += x08
	B[9] += x09
	B[10] += x10
	B[11] += x11
	B[12] += x12
	B[13] += x13
	B[14] += x14
	B[15] += x15
}

// scrypt102411256 matches CPureBlockHeader::GetPoWHash (generic C path).
func scrypt102411256(input80 []byte) []byte {
	if len(input80) != 80 {
		panic("scrypt input must be 80 bytes")
	}
	B := pbkdf2.Key(input80, input80, 1, 128, sha256.New)

	var X [32]uint32
	for k := 0; k < 32; k++ {
		X[k] = binary.LittleEndian.Uint32(B[4*k:])
	}

	V := make([]uint32, scryptScratchWords)

	for i := 0; i < 1024; i++ {
		copy(V[i*32:(i+1)*32], X[:])
		xorSalsa8((*[16]uint32)(X[0:16]), (*[16]uint32)(X[16:32]))
		xorSalsa8((*[16]uint32)(X[16:32]), (*[16]uint32)(X[0:16]))
	}
	for i := 0; i < 1024; i++ {
		j := 32 * int(X[16]&1023)
		for k := 0; k < 32; k++ {
			X[k] ^= V[j+k]
		}
		xorSalsa8((*[16]uint32)(X[0:16]), (*[16]uint32)(X[16:32]))
		xorSalsa8((*[16]uint32)(X[16:32]), (*[16]uint32)(X[0:16]))
	}
	for k := 0; k < 32; k++ {
		binary.LittleEndian.PutUint32(B[4*k:], X[k])
	}

	return pbkdf2.Key(input80, B, 1, 32, sha256.New)
}
