// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"

	"dogego/chain"
	"dogego/wallet"
)

func TestEncryptSetupWallet(t *testing.T) {
	dir := t.TempDir()
	wpath, err := ensureSetupWallet(dir, "testnet")
	if err != nil {
		t.Fatal(err)
	}
	pass := "wizard-test-passphrase"
	if err := encryptSetupWallet(dir, "testnet", pass); err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(wpath, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if !w.IsEncrypted() {
		t.Fatal("wallet should be encrypted after encryptSetupWallet")
	}
	if w.IsUnlocked() {
		t.Fatal("wallet should start locked after encrypt")
	}
	if err := w.Unlock(pass, 60); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if !w.IsUnlocked() {
		t.Fatal("wallet should unlock with passphrase")
	}
	if err := encryptSetupWallet(dir, "testnet", "other"); err != nil {
		t.Fatalf("expected idempotent skip when already encrypted, got: %v", err)
	}
}
