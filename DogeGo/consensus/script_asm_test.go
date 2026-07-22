// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestParseScriptASMDepthEqual(t *testing.T) {
	pub, err := ParseScriptASM("DEPTH 0 EQUAL")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyScriptTest(nil, pub, 0); err != ScriptErrOK {
		t.Fatalf("got %s", err)
	}
}

func TestParseScriptASMIfElse(t *testing.T) {
	pub, err := ParseScriptASM("1 IF 1 ELSE 0 ENDIF")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyScriptTest(nil, pub, 0); err != ScriptErrOK {
		t.Fatalf("got %s", err)
	}
}
