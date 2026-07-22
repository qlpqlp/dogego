// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"dogego/chain"
)

// PowLimitHex is Dogecoin main/test pow limit (same ~uint256(0)>>20).
const PowLimitHex = "00000fffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

// DogeConsensus is the effective consensus rule set at a given height.
type DogeConsensus struct {
	Height                                int64
	PowTargetSpacing                      int64
	PowTargetTimespan                     int64
	Digishield                            bool
	PowAllowMinDifficultyBlocks           bool
	PowAllowDigishieldMinDifficultyBlocks bool
	EnforceStrictMinDifficulty            bool
	AllowLegacyBlocks                     bool
	AuxpowChainID                         int32
	StrictChainID                         bool
	BIP34Height                           int
	BIP65Height                           int
	BIP66Height                           int
	// CSVHeight is when BIP68/112/113 activate (version bits CSV; mainnet block 419328).
	CSVHeight int
	// CoinbaseMaturity is blocks before coinbase outputs may be spent (Core nCoinbaseMaturity).
	CoinbaseMaturity int
}

// DifficultyAdjustmentBlocks returns nPowTargetTimespan / nPowTargetSpacing (Core retarget interval).
func (dc DogeConsensus) DifficultyAdjustmentBlocks() int64 {
	if dc.PowTargetSpacing <= 0 {
		return 1
	}
	n := dc.PowTargetTimespan / dc.PowTargetSpacing
	if n < 1 {
		return 1
	}
	return n
}

// AuxpowActivationHeight is the first block height that must be merge-mined (auxpow), not legacy scrypt.
func AuxpowActivationHeight(net chain.Network) int64 {
	switch net {
	case chain.MainnetDogecoin:
		return 371337
	case chain.RebootTestnet:
		return 158100
	default:
		return 0
	}
}

// LookupConsensus returns Dogecoin Core-equivalent parameters for height.
func LookupConsensus(net chain.Network, height int64) DogeConsensus {
	switch net {
	case chain.MainnetDogecoin:
		switch {
		case height >= 371337:
			return DogeConsensus{
				Height: height, PowTargetSpacing: 60, PowTargetTimespan: 60,
				Digishield: true, PowAllowMinDifficultyBlocks: false,
				PowAllowDigishieldMinDifficultyBlocks: false, EnforceStrictMinDifficulty: false,
				AllowLegacyBlocks: false, AuxpowChainID: 0x0062, StrictChainID: true,
				BIP34Height: 1034383, BIP65Height: 3464751, BIP66Height: 1034383, CSVHeight: 419328,
				CoinbaseMaturity: 240,
			}
		case height >= 145000:
			return DogeConsensus{
				Height: height, PowTargetSpacing: 60, PowTargetTimespan: 60,
				Digishield: true, PowAllowMinDifficultyBlocks: false,
				PowAllowDigishieldMinDifficultyBlocks: false, EnforceStrictMinDifficulty: false,
				AllowLegacyBlocks: true, AuxpowChainID: 0x0062, StrictChainID: true,
				BIP34Height: 1034383, BIP65Height: 3464751, BIP66Height: 1034383, CSVHeight: 419328,
				CoinbaseMaturity: 240,
			}
		default:
			return DogeConsensus{
				Height: height, PowTargetSpacing: 60, PowTargetTimespan: 4 * 60 * 60,
				Digishield: false, PowAllowMinDifficultyBlocks: false,
				PowAllowDigishieldMinDifficultyBlocks: false, EnforceStrictMinDifficulty: false,
				AllowLegacyBlocks: true, AuxpowChainID: 0x0062, StrictChainID: true,
				BIP34Height: 1034383, BIP65Height: 3464751, BIP66Height: 1034383, CSVHeight: 419328,
				CoinbaseMaturity: 30,
			}
		}
	case chain.RebootTestnet:
		if height == 0 {
			return DogeConsensus{
				Height: height, PowTargetSpacing: 60, PowTargetTimespan: 4 * 60 * 60,
				Digishield: false, PowAllowMinDifficultyBlocks: true,
				PowAllowDigishieldMinDifficultyBlocks: false, EnforceStrictMinDifficulty: false,
				AllowLegacyBlocks: true, AuxpowChainID: 0x0062, StrictChainID: false,
				BIP34Height: 708658, BIP65Height: 1854705, BIP66Height: 708658, CSVHeight: 708658,
				CoinbaseMaturity: 30,
			}
		}
		if height < 158100 {
			// Digishield + tail subsidy from block 1; PR #3967 strict min-diff from genesis; legacy scrypt for solo miners.
			return DogeConsensus{
				Height: height, PowTargetSpacing: 60, PowTargetTimespan: 60,
				Digishield: true, PowAllowMinDifficultyBlocks: false,
				PowAllowDigishieldMinDifficultyBlocks: true, EnforceStrictMinDifficulty: true,
				AllowLegacyBlocks: true, AuxpowChainID: 0x0062, StrictChainID: false,
				BIP34Height: 708658, BIP65Height: 1854705, BIP66Height: 708658, CSVHeight: 708658,
				CoinbaseMaturity: 240,
			}
		}
		return DogeConsensus{
			Height: height, PowTargetSpacing: 60, PowTargetTimespan: 60,
			Digishield: true, PowAllowMinDifficultyBlocks: false,
			PowAllowDigishieldMinDifficultyBlocks: true, EnforceStrictMinDifficulty: true,
			AllowLegacyBlocks: false, AuxpowChainID: 0x0062, StrictChainID: false,
			BIP34Height: 708658, BIP65Height: 1854705, BIP66Height: 708658, CSVHeight: 708658,
			CoinbaseMaturity: 240,
		}
	default:
		return DogeConsensus{Height: height, PowTargetSpacing: 60, PowTargetTimespan: 60, AllowLegacyBlocks: true, AuxpowChainID: 0x0062}
	}
}
