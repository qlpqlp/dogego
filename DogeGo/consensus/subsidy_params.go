// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "dogego/chain"

// SubsidyParams holds height-dependent coinbase subsidy rules (Core Consensus::Params).
type SubsidyParams struct {
	SimplifiedRewards     bool
	HalvingInterval       int64
	TailRewardFromHeight1 bool // reboot testnet: 10k DOGE/block from height 1 (mainnet tail)
}

// LookupSubsidyParams returns subsidy parameters for height on Dogecoin networks.
func LookupSubsidyParams(net chain.Network, height int64) SubsidyParams {
	const mainHalving = int64(100_000)
	switch net {
	case chain.RebootTestnet:
		return SubsidyParams{SimplifiedRewards: true, HalvingInterval: mainHalving, TailRewardFromHeight1: true}
	case chain.MainnetDogecoin:
		if height < 145_000 {
			return SubsidyParams{SimplifiedRewards: false, HalvingInterval: mainHalving}
		}
		return SubsidyParams{SimplifiedRewards: true, HalvingInterval: mainHalving}
	default:
		return SubsidyParams{SimplifiedRewards: true, HalvingInterval: mainHalving}
	}
}
