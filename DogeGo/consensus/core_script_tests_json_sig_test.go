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

	"dogego/wire"
)

// TestCoreScriptTestsJSONP2PKSignature verifies auto-generated P2PK row from script_tests.json once sighash matches Core.
func TestCoreScriptTestsJSONP2PKSignature(t *testing.T) {
	sigASM := "0x47 0x304402200a5c6163f07b8d3b013c4d1d6dba25e780b39658d79ba37af7057a3b7f15ffa102201fd9b4eaa9943f734928b99a83592c2e7bf342ea2680f6a2bb705167966b742001"
	pubASM := scriptTestP2PKPubASM
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
	pubKey, err := parseP2PKPubKeyScript(pub)
	if err != nil {
		t.Fatal(err)
	}
	key, err := secp256k1.ParsePubKey(pubKey)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := wire.CalcSignatureHashLegacy(pub, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	der, err := ecdsa.ParseDERSignature(sigBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !der.Verify(digest[:], key) {
		t.Fatal("Core script_tests.json P2PK sig does not verify against legacy sighash (see TestCoreSighashDifferentialHarness)")
	}
	if got := VerifyScriptTestSpend(sig, pub, 0); got != ScriptErrOK {
		t.Fatalf("verify: %s", got)
	}
}
