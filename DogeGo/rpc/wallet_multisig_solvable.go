// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"dogego/chain"
	"dogego/consensus"
)

func walletSolvableFromP2SHRedeem(chainName string, paths *DataPaths, redeem []byte) bool {
	if len(redeem) == 0 {
		return false
	}
	if nReq, pubs, ok := consensus.MultisigRedeemFromP2SH(redeem); ok {
		return descriptorWalletMultisigSolvable(chainName, paths, nReq, pubs)
	}
	inner, ok := consensus.InnerSigningScriptFromP2SHRedeem(redeem)
	if !ok || len(inner) != 25 || inner[0] != 0x76 {
		return false
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return false
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return false
	}
	addr := chain.ScriptPubKeyAddress(inner, p.PubkeyHashAddrID, p.ScriptHashAddrID)
	return descriptorWalletHasSpendKey(paths, addr)
}
