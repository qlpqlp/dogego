// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"dogego/rpc"
	"dogego/wallet"
)

func wireWalletEncryption(paths *rpc.DataPaths, disk *wallet.Disk) {
	if paths == nil || disk == nil {
		return
	}
	paths.WalletIsEncrypted = func() bool { return disk.IsEncrypted() }
	paths.WalletIsUnlocked = func() bool { return disk.IsUnlocked() }
	paths.WalletUnlockUntil = func() int64 { return disk.UnlockUntil() }
	paths.WalletEncrypt = func(pass string) (string, error) { return disk.Encrypt(pass) }
	paths.WalletUnlock = func(pass string, sec int64) error { return disk.Unlock(pass, sec) }
	paths.WalletLock = func() error { return disk.Lock() }
	paths.WalletChangePassphrase = func(oldPass, newPass string) error {
		return disk.ChangePassphrase(oldPass, newPass)
	}
}
