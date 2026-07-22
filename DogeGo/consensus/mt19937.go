// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// mt19937 is a 32-bit Mersenne Twister (Boost mt19937-compatible seeding).
type mt19937 struct {
	mt    [624]uint32
	index int
}

func newMT19937(seed uint32) *mt19937 {
	m := &mt19937{index: 624}
	m.mt[0] = seed
	for i := 1; i < 624; i++ {
		m.mt[i] = 1812433253*(m.mt[i-1]^(m.mt[i-1]>>30)) + uint32(i)
	}
	return m
}

func (m *mt19937) Uint32() uint32 {
	if m.index >= 624 {
		m.twist()
	}
	y := m.mt[m.index]
	y ^= y >> 11
	y ^= (y << 7) & 0x9d2c5680
	y ^= (y << 15) & 0xefc60000
	y ^= y >> 18
	m.index++
	return y
}

func (m *mt19937) twist() {
	const mag01 = uint32(0x9908b0df)
	for i := 0; i < 624; i++ {
		y := (m.mt[i] & 0x80000000) + (m.mt[(i+1)%624] & 0x7fffffff)
		m.mt[i] = m.mt[(i+397)%624] ^ (y >> 1)
		if y&1 != 0 {
			m.mt[i] ^= mag01
		}
	}
	m.index = 0
}

// generateMTRandom matches Core dogecoin.cpp (boost::uniform_int<> dist(1, range)).
func generateMTRandom(seed uint, maxReward int) int {
	if maxReward < 1 {
		return 0
	}
	const (
		engineMin uint64 = 0
		engineMax uint64 = 0xffffffff
	)
	span := uint64(maxReward) // inclusive [1, maxReward] => width maxReward
	brange := engineMax - engineMin + 1
	if span == 1 {
		return 1
	}
	rng := newMT19937(uint32(seed))
	mod := brange / span
	if mod > (brange-span) || mod == 0 {
		// Rejection path (not needed for Dogecoin subsidy: span << 2^32).
		for {
			u := uint64(rng.Uint32())
			if u < span*mod {
				return int(u/mod) + 1
			}
		}
	}
	return int((uint64(rng.Uint32())-engineMin)/mod + 1)
}
