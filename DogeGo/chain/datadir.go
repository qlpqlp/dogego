// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import "fmt"

// ChainDataDirName is the subdirectory under the base data directory for this network
// (similar in spirit to Dogecoin Core's mainnet vs testnet3 layout).
func ChainDataDirName(net Network) (string, error) {
	switch net {
	case MainnetDogecoin:
		return "mainnet", nil
	case RebootTestnet:
		return "testnet", nil
	default:
		return "", fmt.Errorf("unknown network %d", net)
	}
}
