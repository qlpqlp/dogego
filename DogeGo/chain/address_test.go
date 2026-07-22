// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import (
	"encoding/hex"
	"testing"
)

func TestBase58CheckEncode_vector(t *testing.T) {
	// Reboot testnet pubkey prefix 0x41; random-looking hash160 (not a real UTXO).
	hash, err := hex.DecodeString("9131c29384f000c0d651660eefaf1717c8ca1855")
	if err != nil || len(hash) != 20 {
		t.Fatal(err)
	}
	got := Base58CheckEncode(0x41, hash)
	if got == "" {
		t.Fatal("empty")
	}
	if got[0] != 'T' {
		t.Fatalf("expected testnet-style leading char T, got %q", got)
	}
	v, h160, err := Base58CheckDecode(got)
	if err != nil {
		t.Fatal(err)
	}
	if v != 0x41 {
		t.Fatalf("version %x", v)
	}
	if string(h160[:]) != string(hash) {
		t.Fatalf("hash160 mismatch")
	}
}

func TestRandomP2PKHAddress(t *testing.T) {
	p, err := ParamsFor(RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	a, err := RandomP2PKHAddress(p)
	if err != nil || len(a) < 26 || len(a) > 35 {
		t.Fatalf("%q err %v", a, err)
	}
}
