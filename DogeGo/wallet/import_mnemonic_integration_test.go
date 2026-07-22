// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"strings"
	"testing"

	"dogego/chain"
)

func TestRestoreFromMnemonicDeterministicAddress(t *testing.T) {
	dir := t.TempDir()
	pass := byte(0x42)
	w, err := LoadOrCreate(dir+"/wallet.json", pass)
	if err != nil {
		t.Fatal(err)
	}
	m := strings.Repeat("abandon ", 11) + "about"
	if err := w.RestoreFromMnemonic(m, ""); err != nil {
		t.Fatal(err)
	}
	addr1 := w.DefaultAddress()
	if addr1 == "" {
		t.Fatal("empty address")
	}
	params, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	entries := w.ListAddressEntries(params.PubkeyHashAddrID, params.ScriptHashAddrID)
	if len(entries) < 1 {
		t.Fatalf("entries %d", len(entries))
	}
	found := false
	for _, e := range entries {
		if e.Address == addr1 && e.HDPath != "" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("default addr %s not in entries %#v", addr1, entries)
	}
	w2, err := LoadOrCreate(dir+"/wallet.json", pass)
	if err != nil {
		t.Fatal(err)
	}
	if w2.DefaultAddress() != addr1 {
		t.Fatalf("reload addr %s vs %s", w2.DefaultAddress(), addr1)
	}
}

func TestRestoreFromMnemonicWithPassphrase(t *testing.T) {
	dir := t.TempDir()
	p := byte(0x55)
	w, err := LoadOrCreate(dir+"/wallet.json", p)
	if err != nil {
		t.Fatal(err)
	}
	m := "letter advice cage absurd amount doctor acoustic avoid letter advice cage above"
	if err := w.RestoreFromMnemonic(m, "hunter2"); err != nil {
		t.Fatal(err)
	}
	addrPass := w.DefaultAddress()
	w.RestoreFromMnemonic(m, "")
	addrEmpty := w.DefaultAddress()
	if addrPass == addrEmpty {
		t.Fatal("passphrase should change derived address")
	}
}

func TestRestoreFromMnemonicMainnetBIP44Path(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	w, err := LoadOrCreate(dir+"/wallet.json", p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	m := strings.Repeat("abandon ", 11) + "about"
	if err := w.RestoreFromMnemonic(m, ""); err != nil {
		t.Fatal(err)
	}
	addr := w.DefaultAddress()
	if len(addr) < 2 || addr[0] != 'D' {
		t.Fatalf("mainnet P2PKH want D… got %q", addr)
	}
	path, chg, ok := w.AddressHDPath(addr)
	if !ok || chg {
		t.Fatalf("path ok=%v chg=%v", ok, chg)
	}
	if path != "m/44'/3'/0'/0/0" {
		t.Fatalf("path %q want m/44'/3'/0'/0/0", path)
	}
	// Regression lock: BIP39 abandon×11+about → Dogecoin mainnet receive index 0.
	const wantAddr = "DKznsfbYgqKSg6FW1wHhAJxwHF6VSDkHGS"
	if addr != wantAddr {
		t.Fatalf("address %q want %q", addr, wantAddr)
	}
}

func TestRestoreFromMnemonicInvalidChecksum(t *testing.T) {
	dir := t.TempDir()
	w, err := LoadOrCreate(dir+"/wallet.json", byte(0x71))
	if err != nil {
		t.Fatal(err)
	}
	m := strings.Repeat("abandon ", 11) + "ability"
	if err := w.RestoreFromMnemonic(m, ""); err == nil {
		t.Fatal("expected checksum error")
	}
}
