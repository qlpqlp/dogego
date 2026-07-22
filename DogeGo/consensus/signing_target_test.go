// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/secp256k1"
)

func TestSigningScriptAndRedeemP2SHMultisig(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x92
	_, pub := secp256k1.PrivKeyFromBytes(sec)
	redeem := buildTestMultisigRedeem(1, pub.SerializeCompressed())
	p2sh := p2shScriptPubKey(redeem)
	code, push, pushes, err := SigningScriptAndRedeem(p2sh, redeem, nil)
	if err != nil || !push || !IsMultisigRedeemScript(code) || len(pushes) != 1 {
		t.Fatalf("code=%x push=%v pushes=%d err=%v", code, push, len(pushes), err)
	}
}

func TestSigningScriptAndRedeemP2SHP2PKH(t *testing.T) {
	redeem := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	redeem = append(redeem, 0x88, 0xac)
	p2sh := p2shScriptPubKey(redeem)
	code, push, _, err := SigningScriptAndRedeem(p2sh, redeem, nil)
	if err != nil || !push || !isP2PKHScript(code) {
		t.Fatalf("code=%x push=%v err=%v", code, push, err)
	}
}

func TestSigningScriptAndRedeemNestedP2SHP2PKH(t *testing.T) {
	inner := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	inner[3] = 0x33
	inner = append(inner, 0x88, 0xac)
	forward := p2shScriptPubKey(inner)
	outer := p2shScriptPubKey(forward)
	code, push, pushes, err := SigningScriptAndRedeem(outer, forward, inner)
	if err != nil || !push || !isP2PKHScript(code) || len(pushes) != 2 {
		t.Fatalf("code=%x push=%v pushes=%d err=%v", code, push, len(pushes), err)
	}
	if !isP2SHForwardRedeem(pushes[1]) {
		t.Fatalf("outer push %x", pushes[1])
	}
}
