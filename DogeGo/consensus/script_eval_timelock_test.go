// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestEvalCheckLockTimeVerifyNOPWithoutFlag(t *testing.T) {
	pub, _ := ParseScriptASM("1 CHECKLOCKTIMEVERIFY")
	got := VerifyScriptTest(nil, pub, ScriptVerifyMinimalData)
	if got != ScriptErrOK {
		t.Fatalf("CLTV without flag should NOP: %s", got)
	}
}

func TestEvalCheckLockTimeVerifyDiscourage(t *testing.T) {
	pub, _ := ParseScriptASM("1 CHECKLOCKTIMEVERIFY")
	got := VerifyScriptTest(nil, pub, ScriptVerifyDiscourageUpgradableNops)
	if got != ScriptErrDiscourageUpgradable {
		t.Fatalf("got %s", got)
	}
}

func TestEvalCheckLockTimeVerifyActive(t *testing.T) {
	pub, _ := ParseScriptASM("1 CHECKLOCKTIMEVERIFY")
	got := VerifyScriptTest(nil, pub, ScriptVerifyCheckLockTimeVerify)
	if got != ScriptErrUnsatisfiedLocktime {
		t.Fatalf("Core DoTest spend has nLockTime=0 and final sequence; got %s", got)
	}
}

func TestEvalCheckSequenceVerifyDisableOperandNOP(t *testing.T) {
	pub, _ := ParseScriptASM("2147483648 CHECKSEQUENCEVERIFY")
	spend, _ := buildScriptTestCreditSpendFlags(nil, pub, ScriptVerifyCheckSequenceVerify)
	checker := &ScriptSpendChecker{Tx: spend, InputIdx: 0, Subscript: pub}
	_, serr := evalScript(nil, pub, ScriptVerifyCheckSequenceVerify, checker)
	if serr != ScriptErrOK {
		t.Fatalf("disable-flag operand should NOP: %s", serr)
	}
}
