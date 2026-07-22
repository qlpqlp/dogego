// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/wallet"
)

func TestEncryptWalletPaths(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	path := filepath.Join(t.TempDir(), "wallet.json")
	w, _ := wallet.LoadOrCreate(path, p.PubkeyHashAddrID)
	paths := &DataPaths{
		WalletDefaultAddress:   func() string { return w.Address() },
		WalletIsEncrypted:      func() bool { return w.IsEncrypted() },
		WalletIsUnlocked:       func() bool { return w.IsUnlocked() },
		WalletUnlockUntil:      func() int64 { return w.UnlockUntil() },
		WalletEncrypt:          func(pass string) (string, error) { return w.Encrypt(pass) },
		WalletUnlock:           func(pass string, sec int64) error { return w.Unlock(pass, sec) },
		WalletLock:             func() error { return w.Lock() },
		WalletChangePassphrase: func(o, n string) error { return w.ChangePassphrase(o, n) },
	}
	phraseJ, _ := json.Marshal("secret")
	res, code, msg := execEncryptWalletPaths(paths, []json.RawMessage{phraseJ})
	if code != 0 || res == "" {
		t.Fatalf("encrypt %d %s %#v", code, msg, res)
	}
	timeoutJ, _ := json.Marshal(60)
	_, code, msg = execWalletPassphrasePaths(paths, []json.RawMessage{phraseJ, timeoutJ})
	if code != 0 {
		t.Fatalf("unlock %d %s", code, msg)
	}
	_, code, msg = execWalletLockPaths(paths, nil)
	if code != 0 {
		t.Fatalf("lock %d %s", code, msg)
	}
}
