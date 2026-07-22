// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import "testing"

func TestPayToScriptHashAddressRoundTrip(t *testing.T) {
	p, _ := ParamsFor(RebootTestnet)
	var h [20]byte
	h[0] = 0x11
	script := P2SHScriptFromScriptHash(h)
	addr := PayToScriptHashAddress(script, p.ScriptHashAddrID)
	if addr == "" {
		t.Fatal("empty p2sh address")
	}
	v, got, err := Base58CheckDecode(addr)
	if err != nil || v != p.ScriptHashAddrID || got != h {
		t.Fatalf("decode %v %v %v", v, got, err)
	}
	if ScriptPubKeyAddress(script, p.PubkeyHashAddrID, p.ScriptHashAddrID) != addr {
		t.Fatal("ScriptPubKeyAddress mismatch")
	}
}

func TestHash160Len(t *testing.T) {
	if len(Hash160([]byte{1, 2, 3})) != 20 {
		t.Fatal("hash160 len")
	}
}
