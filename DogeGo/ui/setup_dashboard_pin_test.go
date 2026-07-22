// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/ui/websecurity"
	"dogego/wallet"
)

func TestApplySetupDashboardPINDual(t *testing.T) {
	dir := t.TempDir()
	if err := applySetupDashboardPINNetworks(dir, []string{"mainnet", "testnet"}, "123456"); err != nil {
		t.Fatal(err)
	}
	for _, net := range []string{"mainnet", "testnet"} {
		n, err := chain.ParseNetwork(net)
		if err != nil {
			t.Fatal(err)
		}
		sub, err := chain.ChainDataDirName(n)
		if err != nil {
			t.Fatal(err)
		}
		g, err := websecurity.NewGate(filepath.Join(dir, sub))
		if err != nil {
			t.Fatal(err)
		}
		if !g.Enabled() {
			t.Fatalf("%s: dashboard PIN not enabled", net)
		}
	}
}

func TestEncryptSetupWalletMainnetAndTestnet(t *testing.T) {
	dir := t.TempDir()
	pass := "dual-wizard-pass"
	for _, net := range []string{"mainnet", "testnet"} {
		if _, err := ensureSetupWallet(dir, net); err != nil {
			t.Fatal(err)
		}
		if err := encryptSetupWallet(dir, net, pass); err != nil {
			t.Fatalf("encrypt %s: %v", net, err)
		}
	}
	for _, net := range []string{"mainnet", "testnet"} {
		n, err := chain.ParseNetwork(net)
		if err != nil {
			t.Fatal(err)
		}
		p, err := chain.ParamsFor(n)
		if err != nil {
			t.Fatal(err)
		}
		sub, err := chain.ChainDataDirName(n)
		if err != nil {
			t.Fatal(err)
		}
		wpath := filepath.Join(dir, sub, "wallet.json")
		w, err := wallet.LoadOrCreate(wpath, p.PubkeyHashAddrID)
		if err != nil {
			t.Fatal(err)
		}
		if !w.IsEncrypted() {
			t.Fatalf("%s wallet should be encrypted", net)
		}
	}
}
