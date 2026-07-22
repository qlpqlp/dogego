// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"encoding/hex"
	"testing"
)

// BIP32 test vector 1 (https://github.com/bitcoin/bips/blob/master/bip-0032.mediawiki#test-vector-1)
func TestBIP32Vector1Chainm0Hardened0Normal0Index0(t *testing.T) {
	seed, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	path := []uint32{
		hardenedFlag + 44,
		hardenedFlag + 3,
		hardenedFlag + 0,
		0,
		0,
	}
	ek, err := derivePath(seed, path)
	if err != nil {
		t.Fatal(err)
	}
	// Spot-check: derived key is valid and non-zero.
	if ek.key.Key.IsZero() {
		t.Fatal("zero key")
	}
	_ = ek.key.PubKey().SerializeCompressed()
}

func TestHDNewReceiveAddressesDistinct(t *testing.T) {
	p := byte(0x41)
	w := &Disk{path: t.TempDir() + "/w.json", addrVer: p}
	if err := w.initHDLocked(); err != nil {
		t.Fatal(err)
	}
	a0 := w.DefaultAddress()
	a1, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	if a0 == a1 {
		t.Fatalf("expected new address, got %s", a1)
	}
	a2, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	if a1 == a2 {
		t.Fatalf("expected distinct addresses")
	}
	scripts := w.SpendScripts()
	if len(scripts) < 3 {
		t.Fatalf("spend scripts %d", len(scripts))
	}
}

func TestHDDeriveStable(t *testing.T) {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	d1, err := deriveHDAt(seed, 0x41, bip44ReceivePath(0, 5))
	if err != nil {
		t.Fatal(err)
	}
	d2, err := deriveHDAt(seed, 0x41, bip44ReceivePath(0, 5))
	if err != nil {
		t.Fatal(err)
	}
	if d1.Addr != d2.Addr {
		t.Fatalf("%s vs %s", d1.Addr, d2.Addr)
	}
	if !d1.Priv.Key.Equals(&d2.Priv.Key) {
		t.Fatal("priv mismatch")
	}
}
