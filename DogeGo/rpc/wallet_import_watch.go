// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "fmt"

// rpcWalletImportWatchScript imports a watch scriptPubKey and optionally stores redeemScript for P2SH signing.
func rpcWalletImportWatchScript(paths *DataPaths, pkScript, redeem []byte) error {
	if paths == nil || paths.WalletImportWatch == nil {
		return fmt.Errorf("wallet not available")
	}
	if err := paths.WalletImportWatch(pkScript); err != nil {
		return err
	}
	if len(redeem) > 0 && paths.WalletSetWatchRedeem != nil {
		return paths.WalletSetWatchRedeem(pkScript, redeem)
	}
	return nil
}
