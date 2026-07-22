// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "sync/atomic"

var walletScanning atomic.Bool

// WalletIsScanning reports whether a wallet block rescan is in progress.
func WalletIsScanning() bool {
	return walletScanning.Load()
}

func setWalletScanning(on bool) {
	walletScanning.Store(on)
}
