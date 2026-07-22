// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestParseScriptASMMinimalDataDrop(t *testing.T) {
	sig, err := ParseScriptASM("0x01 0x02")
	if err != nil {
		t.Fatal(err)
	}
	pub, err := ParseScriptASM("DROP 1")
	if err != nil {
		t.Fatal(err)
	}
	got := VerifyScriptTest(sig, pub, ScriptVerifyMinimalData)
	if got != ScriptErrMinimalData {
		t.Fatalf("got %s want MINIMALDATA", got)
	}
}

func TestParseScriptASMOverflowAdd(t *testing.T) {
	sig, err := ParseScriptASM("2147483648 0 ADD")
	if err != nil {
		t.Fatal(err)
	}
	_, serr := evalScript(nil, sig, 0, nil)
	if serr != ScriptErrUnknown {
		t.Fatalf("got %s want UNKNOWN_ERROR", serr)
	}
}

func TestNestedIFScriptTest(t *testing.T) {
	sig, err := ParseScriptASM("1 0")
	if err != nil {
		t.Fatal(err)
	}
	pub, err := ParseScriptASM("IF IF 1 ELSE 0 ENDIF ENDIF")
	if err != nil {
		t.Fatal(err)
	}
	spend, _ := buildScriptTestCreditSpendFlags(sig, pub, 0)
	checker := &ScriptSpendChecker{Tx: spend, InputIdx: 0, Subscript: pub}
	stack, serr := evalScript(nil, sig, 0, checker)
	if serr != ScriptErrOK {
		t.Fatalf("sig: %s", serr)
	}
	stack, serr = evalScript(stack, pub, 0, checker)
	if serr != ScriptErrOK {
		t.Fatalf("pub: %s", serr)
	}
	if len(stack) == 0 || !scriptCastToBool(stack[len(stack)-1]) {
		t.Fatalf("stack=%v", stack)
	}
}

func TestIfCatSkippedWhenFalse(t *testing.T) {
	sig, err := ParseScriptASM("'a' 'b' 0")
	if err != nil {
		t.Fatal(err)
	}
	pub, err := ParseScriptASM("IF CAT ELSE 1 ENDIF")
	if err != nil {
		t.Fatal(err)
	}
	flags := ParseScriptTestFlags("P2SH,STRICTENC") | ScriptVerifyP2SH
	if got := VerifyScriptTest(sig, pub, flags); got != ScriptErrDisabledOpcode {
		t.Fatalf("got %s want DISABLED_OPCODE (Core script_tests.json)", got)
	}
}
