// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"path/filepath"
	"testing"

	"dogego/chain"
)

func TestEncryptUnlockLockRoundTrip(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "wallet.json")
	w, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	addr := w.Address()
	msg, err := w.Encrypt("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if msg == "" {
		t.Fatal("expected encrypt message")
	}
	if !w.IsEncrypted() || w.IsUnlocked() {
		t.Fatalf("encrypted=%v unlocked=%v", w.IsEncrypted(), w.IsUnlocked())
	}
	if _, err := w.WIFExport(p.PrivKeyWIFVersion); err != ErrWalletLocked {
		t.Fatalf("wif: %v", err)
	}
	if err := w.Unlock("hunter2", 120); err != nil {
		t.Fatal(err)
	}
	if !w.IsUnlocked() || w.Address() != addr {
		t.Fatalf("addr %s unlocked=%v", w.Address(), w.IsUnlocked())
	}
	wif, err := w.WIFExport(p.PrivKeyWIFVersion)
	if err != nil || wif == "" {
		t.Fatalf("wif %v", err)
	}
	if err := w.Lock(); err != nil {
		t.Fatal(err)
	}
	w2, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if !w2.IsEncrypted() || w2.IsUnlocked() {
		t.Fatal("reload should be locked")
	}
	if err := w2.Unlock("hunter2", 0); err != nil {
		t.Fatal(err)
	}
	if w2.Address() != addr {
		t.Fatalf("addr %s != %s", w2.Address(), addr)
	}
}

func TestChangePassphrase(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	path := filepath.Join(t.TempDir(), "wallet.json")
	w, _ := LoadOrCreate(path, p.PubkeyHashAddrID)
	_, _ = w.Encrypt("old")
	_ = w.Unlock("old", 0)
	if err := w.ChangePassphrase("old", "new"); err != nil {
		t.Fatal(err)
	}
	_ = w.Lock()
	w2, _ := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err := w2.Unlock("new", 0); err != nil {
		t.Fatal(err)
	}
	if err := w2.Unlock("old", 0); err == nil {
		t.Fatal("old passphrase should fail")
	}
}
