// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// SipHash24 returns SipHash-2-4 matching Bitcoin Core CSipHasher (blockfilter.cpp / crypto/siphash.cpp).
func SipHash24(k0, k1 uint64, data []byte) uint64 {
	const (
		c0 = 0x736f6d6570736575
		c1 = 0x646f72616e646f6d
		c2 = 0x6c7967656e657261
		c3 = 0x7465646279746573
	)
	h := sipHasher{
		v: [4]uint64{c0 ^ k0, c1 ^ k1, c2 ^ k0, c3 ^ k1},
	}
	h.write(data)
	return h.finalize()
}

type sipHasher struct {
	v     [4]uint64
	count uint8
	tmp   uint64
}

func (h *sipHasher) write(data []byte) {
	v0, v1, v2, v3 := h.v[0], h.v[1], h.v[2], h.v[3]
	t := h.tmp
	c := h.count
	for len(data) > 0 {
		t |= uint64(data[0]) << (8 * (c % 8))
		c++
		if c&7 == 0 {
			v3 ^= t
			v0, v1, v2, v3 = sipRound(v0, v1, v2, v3)
			v0, v1, v2, v3 = sipRound(v0, v1, v2, v3)
			v0 ^= t
			t = 0
		}
		data = data[1:]
	}
	h.v = [4]uint64{v0, v1, v2, v3}
	h.count = c
	h.tmp = t
}

func (h *sipHasher) finalize() uint64 {
	v0, v1, v2, v3 := h.v[0], h.v[1], h.v[2], h.v[3]
	t := h.tmp | (uint64(h.count) << 56)
	v3 ^= t
	v0, v1, v2, v3 = sipRound(v0, v1, v2, v3)
	v0, v1, v2, v3 = sipRound(v0, v1, v2, v3)
	v0 ^= t
	v2 ^= 0xff
	for i := 0; i < 4; i++ {
		v0, v1, v2, v3 = sipRound(v0, v1, v2, v3)
	}
	return v0 ^ v1 ^ v2 ^ v3
}

func sipRound(v0, v1, v2, v3 uint64) (uint64, uint64, uint64, uint64) {
	v0 += v1
	v1 = rotl64(v1, 13)
	v1 ^= v0
	v0 = rotl64(v0, 32)
	v2 += v3
	v3 = rotl64(v3, 16)
	v3 ^= v2
	v0 += v3
	v3 = rotl64(v3, 21)
	v3 ^= v0
	v2 += v1
	v1 = rotl64(v1, 17)
	v1 ^= v2
	v2 = rotl64(v2, 32)
	return v0, v1, v2, v3
}

func rotl64(x uint64, b uint) uint64 {
	return (x << b) | (x >> (64 - b))
}
