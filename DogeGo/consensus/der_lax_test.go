// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"testing"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/wire"
)

// Row from script_tests.json: P2PK with too much R padding but no DERSIG.
func TestDerLaxAutogenP2PKPadding(t *testing.T) {
	sigASM := "0x47 0x304402200060558477337b9022e70534f1fea71a318caf836812465a2509931c5e7c4987022078ec32bd50ac9e03a349ba953dfd9fe1c8d2dd8bdb1d38ddca844d3d5c78c11801"
	pubASM := "0x21 0x038282263212c609d9ea2a6e3e172de238d8c39cabd5ac1ca10646e23fd5f51508 CHECKSIG"
	sig, err := ParseScriptASM(sigASM)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := ParseScriptASM(pubASM)
	if err != nil {
		t.Fatal(err)
	}
	spend, _ := buildScriptTestCreditSpend(sig, pub)
	push, err := parseSinglePush(sig)
	if err != nil {
		t.Fatal(err)
	}
	sigBytes, _, err := parseSignaturePush(push)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := parseP2PKPubKeyScript(pub)
	if err != nil {
		t.Fatal(err)
	}
	key, err := secp256k1.ParsePubKey(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := wire.CalcSignatureHashLegacy(pub, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ecdsa.ParseDERSignature(sigBytes); err == nil {
		t.Log("strict DER parse unexpectedly succeeded")
	}
	if !verifyECDSASignatureLax(sigBytes, digest[:], key) {
		t.Fatal("lax verify failed")
	}
	if got := VerifyScriptTestSpend(sig, pub, 0); got != ScriptErrOK {
		t.Fatalf("VerifyScriptTestSpend: %s", got)
	}
}

func TestDerLaxToCompactVectors(t *testing.T) {
	// Canonical minimal DER (r=1, s=2).
	compact, parsed := derLaxToCompact([]byte{0x30, 0x06, 0x02, 0x01, 0x01, 0x02, 0x01, 0x02})
	if !parsed {
		t.Fatal("canonical DER")
	}
	if compact[31] != 0x01 || compact[63] != 0x02 {
		t.Fatalf("compact r/s got %x", compact)
	}
	if isValidSignatureEncoding(append([]byte{0x30, 0x06, 0x02, 0x01, 0x01, 0x02, 0x01, 0x02}, 0x01)) {
		t.Log("canonical DER is also strict-valid with sighash byte")
	}

	// script_tests: too much R padding - lax accepts, strict rejects.
	paddedR := []byte{
		0x30, 0x44, 0x02, 0x20, 0x00, 0x60, 0x55, 0x84, 0x77, 0x33, 0x7b, 0x90, 0x22, 0xe7, 0x05, 0x34,
		0xf1, 0xfe, 0xa7, 0x1a, 0x31, 0x8c, 0xaf, 0x83, 0x68, 0x12, 0x46, 0x5a, 0x25, 0x09, 0x93, 0x1c,
		0x5e, 0x7c, 0x49, 0x87, 0x02, 0x20, 0x78, 0xec, 0x32, 0xbd, 0x50, 0xac, 0x9e, 0x03, 0xa3, 0x49,
		0xba, 0x95, 0x3d, 0xfd, 0x9f, 0xe1, 0xc8, 0xd2, 0xdd, 0x8b, 0xdb, 0x1d, 0x38, 0xdd, 0xca, 0x84,
		0x4d, 0x3d, 0x5c, 0x78, 0xc1, 0x18,
	}
	if _, ok := derLaxToCompact(paddedR); !ok {
		t.Fatal("padded R should parse lax")
	}
	if isValidSignatureEncoding(append(append([]byte(nil), paddedR...), 0x01)) {
		t.Fatal("padded R must fail strict DER")
	}

	// Outer sequence length intentionally wrong (lax skips extra length bytes).
	wrongOuterLen := []byte{0x30, 0x82, 0x00, 0x00, 0x02, 0x01, 0x01, 0x02, 0x01, 0x02}
	if _, ok := derLaxToCompact(wrongOuterLen); !ok {
		t.Fatal("lax should tolerate garbage outer length encoding")
	}

	// R integer > 32 bytes after leading-zero strip → overflow.
	overflowR := make([]byte, 0, 80)
	overflowR = append(overflowR, 0x30, 0x45, 0x02, 0x21)
	overflowR = append(overflowR, bytes.Repeat([]byte{0x01}, 33)...)
	overflowR = append(overflowR, 0x02, 0x01, 0x01)
	if _, ok := derLaxToCompact(overflowR); ok {
		t.Fatal("overflow R should fail")
	}

	// Missing sequence tag.
	if _, ok := derLaxToCompact([]byte{0x31, 0x06, 0x02, 0x01, 0x01, 0x02, 0x01, 0x02}); ok {
		t.Fatal("bad tag")
	}
	// Multi-byte length field with lenbyte >= 8 (Core sizeof(size_t) guard) must fail.
	longLen := []byte{0x30, 0x82, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x01, 0x01, 0x02, 0x01, 0x02}
	if _, ok := derLaxToCompact(longLen); ok {
		t.Fatal("lenbyte>=8 R length should fail lax parse")
	}
	longLenS := []byte{0x30, 0x08, 0x02, 0x01, 0x01, 0x02, 0x82, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
	if _, ok := derLaxToCompact(longLenS); ok {
		t.Fatal("lenbyte>=8 S length should fail lax parse")
	}
}

func TestDerLaxRejectsWithDERSIGFlag(t *testing.T) {
	sigASM := "0x47 0x304402200060558477337b9022e70534f1fea71a318caf836812465a2509931c5e7c4987022078ec32bd50ac9e03a349ba953dfd9fe1c8d2dd8bdb1d38ddca844d3d5c78c11801"
	pubASM := "0x21 0x038282263212c609d9ea2a6e3e172de238d8c39cabd5ac1ca10646e23fd5f51508 CHECKSIG"
	sig, err := ParseScriptASM(sigASM)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := ParseScriptASM(pubASM)
	if err != nil {
		t.Fatal(err)
	}
	if got := VerifyScriptTestSpend(sig, pub, ScriptVerifyDERSig); got != ScriptErrSigDer {
		t.Fatalf("DERSIG on padded sig: got %s want SIG_DER", got)
	}
}
