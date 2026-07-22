// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"testing"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/wire"
)

// TestCoreScriptTestLayoutSighashRoundtrip locks in the script_tests DoTest credit/spend sighash DogeGo uses for signing.
func TestCoreScriptTestLayoutSighashRoundtrip(t *testing.T) {
	pub, err := ParseScriptASM(scriptTestP2PKPubASM)
	if err != nil {
		t.Fatal(err)
	}
	spend, credit := buildScriptTestCreditSpend(nil, pub)
	digest, err := wire.CalcSignatureHashLegacy(pub, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	priv, _ := secp256k1.PrivKeyFromBytes(scriptTestKey0)
	sig := append(ecdsa.Sign(priv, digest[:]).Serialize(), byte(wire.SigHashAll))
	spend.Vin[0].Script = buildSinglePushScript(sig)
	if err := verifyInputP2PK(spend, 0, pub, 0); err != nil {
		t.Fatalf("template verify: %v", err)
	}
	ch := credit.TxHash()
	t.Logf("credit hash=%s spend ser len=%d", hex.EncodeToString(ch[:]), len(spend.SerializeForHash()))
}

// TestCoreScriptTestsJSONSigProbe documents whether Core JSON P2PK sig matches any common prevout layout variant.
func TestCoreScriptTestsJSONSigProbe(t *testing.T) {
	sigASM := "0x47 0x304402200a5c6163f07b8d3b013c4d1d6dba25e780b39658d79ba37af7057a3b7f15ffa102201fd9b4eaa9943f734928b99a83592c2e7bf342ea2680f6a2bb705167966b742001"
	sig, _ := ParseScriptASM(sigASM)
	pub, _ := ParseScriptASM(scriptTestP2PKPubASM)
	push, _ := parseSinglePush(sig)
	sigBytes, _, _ := parseSignaturePush(push)
	pubKey, _ := parseP2PKPubKeyScript(pub)
	key, _ := secp256k1.ParsePubKey(pubKey)
	der, _ := ecdsa.ParseDERSignature(sigBytes)

	spend, _ := buildScriptTestCreditSpend(sig, pub)
	if !tryJSONSig(t, der, key, pub, spend) {
		t.Fatal("Core script_tests.json P2PK sig must verify against DoTest credit/spend layout")
	}
}

func tryJSONSig(t *testing.T, der *ecdsa.Signature, key *secp256k1.PublicKey, pub []byte, spend *wire.Tx) bool {
	t.Helper()
	digest, err := wire.CalcSignatureHashLegacy(pub, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	return der.Verify(digest[:], key)
}
