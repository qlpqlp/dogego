// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "strings"

// rpcWalletDefaultChangeAddress returns the next change address without consuming the keypool (Core fundrawtransaction peek).
func rpcWalletDefaultChangeAddress(paths *DataPaths) string {
	if paths != nil && paths.WalletPeekChangeAddress != nil {
		if c := strings.TrimSpace(paths.WalletPeekChangeAddress()); c != "" {
			return c
		}
	}
	if paths != nil && paths.WalletNewChangeAddress != nil {
		if c, err := paths.WalletNewChangeAddress(); err == nil {
			if c = strings.TrimSpace(c); c != "" {
				return c
			}
		}
	}
	return rpcWalletDefaultAddress(paths)
}
