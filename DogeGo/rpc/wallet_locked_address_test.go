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

func TestGetNewAddressLocked(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	path := filepath.Join(t.TempDir(), "wallet.json")
	w, _ := wallet.LoadOrCreate(path, p.PubkeyHashAddrID)
	_, _ = w.Encrypt("pass")
	paths := &DataPaths{
		WalletDefaultAddress: func() string { return w.Address() },
		WalletNewAddress:     func() (string, error) { return w.NewReceiveAddress() },
		WalletIsEncrypted:    func() bool { return w.IsEncrypted() },
		WalletIsUnlocked:     func() bool { return w.IsUnlocked() },
	}
	_, code, msg := execGetNewAddress("test", paths, nil)
	if code != -13 || msg != walletLockedRPCMsg {
		t.Fatalf("got %d %q want -13 locked", code, msg)
	}
}

func TestSignMessageLocked(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	path := filepath.Join(t.TempDir(), "wallet.json")
	w, _ := wallet.LoadOrCreate(path, p.PubkeyHashAddrID)
	_, _ = w.Encrypt("pass")
	paths := &DataPaths{
		WalletDefaultAddress:  func() string { return w.Address() },
		WalletContainsAddress: func(a string) bool { return w.ContainsAddress(a) },
		WalletIsEncrypted:     func() bool { return w.IsEncrypted() },
		WalletIsUnlocked:      func() bool { return w.IsUnlocked() },
	}
	addrJ, _ := json.Marshal(w.Address())
	msgJ, _ := json.Marshal("hi")
	_, code, got := execSignMessage("test", paths, []json.RawMessage{addrJ, msgJ})
	if code != -13 || got != walletLockedRPCMsg {
		t.Fatalf("got %d %q", code, got)
	}
}
