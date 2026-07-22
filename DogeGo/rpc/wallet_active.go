// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "strings"

// WalletActive reports whether the built-in wallet hooks are wired on DataPaths.
func WalletActive(paths *DataPaths) bool {
	if paths == nil {
		return false
	}
	if paths.WalletImportWatch != nil || paths.WalletImportPrivKey != nil {
		return true
	}
	if paths.WalletDefaultAddress != nil && strings.TrimSpace(paths.WalletDefaultAddress()) != "" {
		return true
	}
	if paths.WalletAddress != nil && strings.TrimSpace(paths.WalletAddress()) != "" {
		return true
	}
	if paths.WalletSpendScripts != nil && len(paths.WalletSpendScripts()) > 0 {
		return true
	}
	if paths.WalletWatchScripts != nil && len(paths.WalletWatchScripts()) > 0 {
		return true
	}
	return false
}
