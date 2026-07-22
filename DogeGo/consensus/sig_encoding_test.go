// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/chain"
	"dogego/wire"
)

func TestIsLowDERSignature_acceptsNormalSign(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x01
	priv, _ := secp256k1.PrivKeyFromBytes(sec)
	msg := [32]byte{0xab}
	sig := ecdsa.Sign(priv, msg[:])
	der := append(sig.Serialize(), byte(wire.SigHashAll))
	if !isLowDERSignature(der) {
		t.Fatal("expected low-S signature from secp256k1.Sign")
	}
}

func TestCheckSignatureEncodingRejectsBadHashType(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x02
	priv, _ := secp256k1.PrivKeyFromBytes(sec)
	var msg [32]byte
	msg[0] = 1
	sig := ecdsa.Sign(priv, msg[:])
	der := append(sig.Serialize(), 0x00) // invalid base hashtype
	err := checkSignatureEncoding(der, ScriptVerifyStrictEnc|ScriptVerifyLowS|ScriptVerifyDERSig)
	if err == nil {
		t.Fatal("expected undefined hashtype error")
	}
}

func TestCheckSignatureEncodingDERSIGRejectsNonCanonical(t *testing.T) {
	// script_tests: too much R padding - valid lax, invalid strict.
	padded := []byte{
		0x30, 0x44, 0x02, 0x20, 0x00, 0x60, 0x55, 0x84, 0x77, 0x33, 0x7b, 0x90, 0x22, 0xe7, 0x05, 0x34,
		0xf1, 0xfe, 0xa7, 0x1a, 0x31, 0x8c, 0xaf, 0x83, 0x68, 0x12, 0x46, 0x5a, 0x25, 0x09, 0x93, 0x1c,
		0x5e, 0x7c, 0x49, 0x87, 0x02, 0x20, 0x78, 0xec, 0x32, 0xbd, 0x50, 0xac, 0x9e, 0x03, 0xa3, 0x49,
		0xba, 0x95, 0x3d, 0xfd, 0x9f, 0xe1, 0xc8, 0xd2, 0xdd, 0x8b, 0xdb, 0x1d, 0x38, 0xdd, 0xca, 0x84,
		0x4d, 0x3d, 0x5c, 0x78, 0xc1, 0x18, byte(wire.SigHashAll),
	}
	if err := checkSignatureEncoding(padded, ScriptVerifyDERSig); err == nil {
		t.Fatal("expected non-DER error with DERSIG")
	}
	if err := checkSignatureEncoding(padded, 0); err != nil {
		t.Fatalf("no flags should skip encoding check: %v", err)
	}
}

func TestIsValidSignatureEncodingRejectsBIP66Violations(t *testing.T) {
	cases := []struct {
		name string
		der  []byte
	}{
		{"missing S", []byte{0x30, 0x22, 0x02, 0x20, 0x00}},
		{"zero-length R with DERSIG intent", []byte{0x30, 0x14, 0x02, 0x00, 0x02, 0x10, 0x77}},
		{"non-integer R", []byte{0x30, 0x24, 0x02, 0x03, 0x10, 0x77, 0x02, 0x10, 0x77}},
	}
	for _, tc := range cases {
		if isValidSignatureEncoding(append(append([]byte(nil), tc.der...), byte(wire.SigHashAll))) {
			t.Fatalf("%s: expected invalid strict DER", tc.name)
		}
	}
}

func TestScriptFlagsForMempoolIncludesStandardPolicy(t *testing.T) {
	f := ScriptFlagsForMempool(5_000_000, chain.MainnetDogecoin, nil)
	if f&ScriptVerifyLowS == 0 || f&ScriptVerifyNullFail == 0 || f&ScriptVerifyStrictEnc == 0 || f&ScriptVerifyMinimalData == 0 ||
		f&ScriptVerifyNullDummy == 0 || f&ScriptVerifyDiscourageUpgradableNops == 0 {
		t.Fatalf("missing standard flags: %x", f)
	}
	if f&ScriptVerifyCheckSequenceVerify == 0 {
		t.Fatal("expected CSV flag at height 5M")
	}
}
