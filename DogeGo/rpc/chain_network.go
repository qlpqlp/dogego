// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "dogego/chain"

func networkFromChainName(chainName string) chain.Network {
	n, err := chain.ParseNetwork(chainName)
	if err != nil {
		return chain.RebootTestnet
	}
	return n
}
