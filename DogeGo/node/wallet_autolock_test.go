// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"dogego/chain"
	"dogego/wallet"
)

func TestWalletAutoLockExpires(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	path := filepath.Join(t.TempDir(), "wallet.json")
	w, _ := wallet.LoadOrCreate(path, p.PubkeyHashAddrID)
	_, _ = w.Encrypt("secret")
	_ = w.Unlock("secret", 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runWalletAutoLock(ctx, w)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !w.IsUnlocked() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("wallet still unlocked after timeout")
}
