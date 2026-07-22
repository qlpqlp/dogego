// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import "fmt"

// ParseNetwork maps CLI strings to a Network id.
func ParseNetwork(s string) (Network, error) {
	switch s {
	case "testnet":
		return RebootTestnet, nil
	case "reboottestnet":
		return RebootTestnet, nil
	case "mainnet", "main":
		return MainnetDogecoin, nil
	default:
		return 0, fmt.Errorf("unknown network %q (use testnet|mainnet)", s)
	}
}
