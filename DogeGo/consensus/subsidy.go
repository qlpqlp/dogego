// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"strconv"

	"dogego/chain"
	"dogego/pow"
)

// KoinuPerCoin is 1 DOGE in koinu (same as COIN in Core).
const KoinuPerCoin = 100_000_000

// BlockSubsidy returns the maximum allowed coinbase subsidy at height (Core GetDogecoinBlockSubsidy).
func BlockSubsidy(height int64, prevHash [32]byte, net chain.Network) int64 {
	sp := LookupSubsidyParams(net, height)
	if sp.TailRewardFromHeight1 && height >= 1 {
		return 10_000 * KoinuPerCoin
	}
	if sp.SimplifiedRewards {
		return BlockSubsidySimplified(height, sp.HalvingInterval)
	}
	return blockSubsidyLegacy(height, sp.HalvingInterval, prevHash)
}

func blockSubsidyLegacy(height int64, halvingInterval int64, prevHash [32]byte) int64 {
	if halvingInterval <= 0 {
		halvingInterval = 100_000
	}
	halvings := height / halvingInterval
	maxReward := (1_000_000 >> halvings) - 1
	if maxReward < 0 {
		maxReward = 0
	}
	display := pow.LEUint256DisplayHex(prevHash[:])
	if len(display) < 14 {
		return 0
	}
	seed, err := strconv.ParseInt(display[7:14], 16, 64)
	if err != nil {
		return 0
	}
	rand := generateMTRandom(uint(seed), int(maxReward))
	return int64(1+rand) * KoinuPerCoin
}

// BlockSubsidySimplified returns the coinbase subsidy when fSimplifiedRewards is active
// (Digishield and later on Dogecoin chains). Matches Core GetDogecoinBlockSubsidy for that era.
func BlockSubsidySimplified(height int64, halvingInterval int64) int64 {
	if halvingInterval <= 0 {
		halvingInterval = 100_000
	}
	halvings := height / halvingInterval
	if height < 6*halvingInterval {
		return (500_000 * KoinuPerCoin) >> halvings
	}
	return 10_000 * KoinuPerCoin
}

// CoinbaseHeightFromScript extracts BIP34 height from the coinbase script if present.
func CoinbaseHeightFromScript(script []byte) (int64, bool) {
	if len(script) < 1 {
		return 0, false
	}
	n := int(script[0])
	if n >= 1 && n <= 4 && len(script) >= 1+n {
		var h int64
		for i := 0; i < n; i++ {
			h |= int64(script[1+i]) << (8 * i)
		}
		return h, true
	}
	if len(script) >= 5 && script[0] == 4 {
		return int64(binary.LittleEndian.Uint32(script[1:5])), true
	}
	return 0, false
}
