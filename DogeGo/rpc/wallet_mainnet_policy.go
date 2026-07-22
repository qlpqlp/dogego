// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"errors"

	"dogego/wallet"
)

const mainnetUnencryptedRPCMsg = "Error: mainnet wallet must be encrypted before spending or exporting keys (encryptwallet)."

func rpcWalletRequireMainnetEncrypted(chainName string, paths *DataPaths) (int, string) {
	if !wallet.MainnetRequiresEncryption(chainName) {
		return 0, ""
	}
	if rpcWalletIsEncrypted(paths) {
		return 0, ""
	}
	return -15, mainnetUnencryptedRPCMsg
}

func rpcWalletMainnetOpErr(err error) (int, string) {
	if errors.Is(err, wallet.ErrMainnetUnencrypted) {
		return -15, mainnetUnencryptedRPCMsg
	}
	return -1, err.Error()
}
