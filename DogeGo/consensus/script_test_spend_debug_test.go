// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/wire"
)

func TestScriptTestChecksigRoundtrip(t *testing.T) {
	pub, err := ParseScriptASM(scriptTestP2PKPubASM)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := signScriptTestSpend(scriptTestKey0, pub, wire.SigHashAll)
	if err != nil {
		t.Fatal(err)
	}
	if got := VerifyScriptTestSpend(sig, pub, 0); got != ScriptErrOK {
		t.Fatalf("P2PK: %s", got)
	}
	bad := append([]byte(nil), sig...)
	bad[len(bad)/2] ^= 0x55
	if got := VerifyScriptTestSpend(bad, pub, 0); got != ScriptErrEvalFalse {
		t.Fatalf("bad sig want EVAL_FALSE got %s", got)
	}
}

func TestScriptTestSpendSighashStableWithScriptSig(t *testing.T) {
	pub, _ := ParseScriptASM(scriptTestP2PKPubASM)
	sig, _ := ParseScriptASM("0x47 0x304402200a5c6163f07b8d3b013c4d1d6dba25e780b39658d79ba37af7057a3b7f15ffa102201fd9b4eaa9943f734928b99a83592c2e7bf342ea2680f6a2bb705167966b742001")
	spendA, _ := buildScriptTestCreditSpend(nil, pub)
	spendB, _ := buildScriptTestCreditSpend(sig, pub)
	dA, _ := wire.CalcSignatureHashLegacy(pub, wire.SigHashAll, spendA, 0)
	dB, _ := wire.CalcSignatureHashLegacy(pub, wire.SigHashAll, spendB, 0)
	if dA != dB {
		t.Fatalf("sighash must not depend on scriptSig payload")
	}
}

