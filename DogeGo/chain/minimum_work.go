// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import (
	"math/big"
	"strings"
)

// minimumChainWorkHex is Core consensus.nMinimumChainWork (display hex, big-endian uint256).
const (
	mainnetMinimumChainWorkHex = "000000000000000000000000000000000000000000000e993d2aa86cf246a49b"
)

var (
	mainnetMinimumChainWork *big.Int
)

func init() {
	mainnetMinimumChainWork, _ = new(big.Int).SetString(mainnetMinimumChainWorkHex, 16)
}

// MinimumChainWork returns Core nMinimumChainWork for the network (nil if none / zero).
func MinimumChainWork(net Network) *big.Int {
	switch net {
	case MainnetDogecoin:
		if mainnetMinimumChainWork == nil {
			return nil
		}
		return new(big.Int).Set(mainnetMinimumChainWork)
	default:
		return nil
	}
}

// MinimumChainWorkHex returns the Core minimum chain work as lowercase hex (empty if none).
func MinimumChainWorkHex(net Network) string {
	switch net {
	case MainnetDogecoin:
		return mainnetMinimumChainWorkHex
	default:
		return ""
	}
}

// MinimumChainWorkForRPCChain maps RPC chain name (main / test / regtest) to minimum work.
func MinimumChainWorkForRPCChain(chainName string) (*big.Int, bool) {
	switch strings.ToLower(strings.TrimSpace(chainName)) {
	case "main", "mainnet":
		w := MinimumChainWork(MainnetDogecoin)
		return w, w != nil
	default:
		return nil, false
	}
}
