// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestEval1AddAndDisabledCat(t *testing.T) {
	pub, err := ParseScriptASM("1 1ADD 2 EQUAL")
	if err != nil {
		t.Fatal(err)
	}
	if got := VerifyScriptTest(nil, pub, ScriptVerifyMinimalData); got != ScriptErrOK {
		t.Fatalf("1ADD: %s", got)
	}
	pub2, err := ParseScriptASM("'a' 'b' CAT")
	if err != nil {
		t.Fatal(err)
	}
	if got := VerifyScriptTest(nil, pub2, 0); got != ScriptErrDisabledOpcode {
		t.Fatalf("CAT: %s", got)
	}
	pub3, err := ParseScriptASM("NOP CODESEPARATOR 1")
	if err != nil {
		t.Fatal(err)
	}
	if got := VerifyScriptTest([]byte{0x61}, pub3, 0); got != ScriptErrOK {
		t.Fatalf("CODESEPARATOR: %s", got)
	}
}
