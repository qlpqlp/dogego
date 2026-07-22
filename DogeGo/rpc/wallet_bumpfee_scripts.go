// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "encoding/hex"

// rpcWalletBumpFeeSpendScripts returns scripts the wallet may sign (HD spend + solvable watch P2SH).
func rpcWalletBumpFeeSpendScripts(chainName string, paths *DataPaths) [][]byte {
	out := append([][]byte{}, rpcWalletSpendScripts(paths)...)
	seen := walletScriptSet(out)
	for _, pk := range rpcWalletWatchScripts(paths) {
		if _, ok := seen[hex.EncodeToString(pk)]; ok {
			continue
		}
		if walletWatchScriptFundable(chainName, paths, pk) {
			out = append(out, pk)
			seen[hex.EncodeToString(pk)] = struct{}{}
		}
	}
	return out
}
