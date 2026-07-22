// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestEvalCheckMultisigZeroKeys(t *testing.T) {
	pub, err := ParseScriptASM("0 0 0 CHECKMULTISIG VERIFY DEPTH 0 EQUAL")
	if err != nil {
		t.Fatal(err)
	}
	flags := ParseScriptTestFlags("P2SH,STRICTENC")
	flags |= ScriptVerifyP2SH
	if got := VerifyScriptTest(nil, pub, flags); got != ScriptErrOK {
		t.Fatalf("zero-key multisig: %s", got)
	}
}

func TestEvalCheckMultisigDirectStack(t *testing.T) {
	spend, _ := buildScriptTestCreditSpend(nil, nil)
	checker := &ScriptSpendChecker{Tx: spend, InputIdx: 0, Subscript: nil}
	stack := [][]byte{nil, nil, nil}
	var serr ScriptError
	stack, serr = evalCheckMultiSig(stack, 0, checker, false, nil)
	if serr != ScriptErrOK {
		t.Fatalf("multisig: %s", serr)
	}
	if len(stack) != 1 || !scriptCastToBool(stack[0]) {
		t.Fatalf("after multisig depth=%d stack=%v", len(stack), stack)
	}
}

func TestEvalCheckMultisigOneKeyZeroSigs(t *testing.T) {
	pub, err := ParseScriptASM("0 0 0 1 CHECKMULTISIG VERIFY DEPTH 0 EQUAL")
	if err != nil {
		t.Fatal(err)
	}
	flags := ParseScriptTestFlags("P2SH,STRICTENC")
	flags |= ScriptVerifyP2SH
	if got := VerifyScriptTest(nil, pub, flags); got != ScriptErrOK {
		t.Fatalf("1 key 0 sigs: %s", got)
	}
}
