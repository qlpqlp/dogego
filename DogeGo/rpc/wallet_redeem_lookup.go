// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"dogego/chain"
)

// walletRedeemScriptForAddress returns a stored watch redeem for a P2SH address, if any.
func walletRedeemScriptForAddress(paths *DataPaths, addr string, p chain.Params) []byte {
	if paths == nil || paths.WalletWatchRedeemScript == nil {
		return nil
	}
	ver, h160, err := chain.Base58CheckDecode(addr)
	if err != nil || ver != p.ScriptHashAddrID {
		return nil
	}
	spk := chain.P2SHScriptFromScriptHash(h160)
	return paths.WalletWatchRedeemScript(spk)
}
