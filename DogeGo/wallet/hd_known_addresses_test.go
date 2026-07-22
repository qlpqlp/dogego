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

func TestKnownAddressesHD(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "wallet.json")
	w, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if !w.HDEnabled() {
		t.Fatal("expected HD wallet")
	}
	a1, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	a2, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	addrs := w.KnownAddresses(p.PubkeyHashAddrID, p.ScriptHashAddrID)
	seen := make(map[string]bool)
	for _, a := range addrs {
		seen[a] = true
	}
	if !seen[a1] || !seen[a2] {
		t.Fatalf("known addresses missing issued receive: %v have %q %q", addrs, a1, a2)
	}
	if !w.ContainsAddress(a1) {
		t.Fatal("ContainsAddress should include issued receive")
	}
}
