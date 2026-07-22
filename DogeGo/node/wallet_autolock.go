// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"time"

	"dogego/wallet"
)

// runWalletAutoLock locks encrypted wallets when walletpassphrase timeout expires.
func runWalletAutoLock(ctx context.Context, disk *wallet.Disk) {
	if disk == nil || !disk.IsEncrypted() {
		return
	}
	go func() {
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				until := disk.UnlockUntil()
				if until <= 0 {
					continue
				}
				if time.Now().Unix() >= until {
					_ = disk.Lock()
				}
			}
		}
	}()
}
