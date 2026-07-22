// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"

	"dogego/consensus"
)

// walletWatchScriptFundable reports whether a watch-only scriptPubKey is solvable enough to fund spends.
// Core includes solvable imported outputs without requiring includeWatching.
func walletWatchScriptFundable(chainName string, paths *DataPaths, pkScript []byte) bool {
	if paths == nil || paths.WalletWatchRedeemScript == nil || len(pkScript) == 0 {
		return false
	}
	for _, w := range rpcWalletWatchScripts(paths) {
		if bytes.Equal(pkScript, w) {
			redeem := paths.WalletWatchRedeemScript(pkScript)
			if len(redeem) == 0 && consensus.IsMultisigRedeemScript(pkScript) {
				redeem = pkScript
			}
			if len(redeem) == 0 {
				return false
			}
			return walletSolvableFromP2SHRedeem(chainName, paths, redeem)
		}
	}
	return false
}
