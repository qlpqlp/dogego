// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestParseScriptTestFlags(t *testing.T) {
	f := ParseScriptTestFlags("DERSIG,STRICTENC")
	if f&ScriptVerifyDERSig == 0 || f&ScriptVerifyStrictEnc == 0 {
		t.Fatalf("flags=%#x", f)
	}
	if ParseScriptTestFlags("NONE") != 0 {
		t.Fatal("NONE should be zero")
	}
}
