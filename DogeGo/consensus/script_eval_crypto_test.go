// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestEvalCryptoHashes(t *testing.T) {
	// SHA256('') = e3b0c442...
	pub, err := ParseScriptASM("'' SHA256 0x20 0xe3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 EQUAL")
	if err != nil {
		t.Fatal(err)
	}
	if got := VerifyScriptTest(nil, pub, ScriptVerifyMinimalData); got != ScriptErrOK {
		t.Fatalf("SHA256 empty: %s", got)
	}
	// RIPEMD160('a')
	pub2, err := ParseScriptASM("'a' RIPEMD160 0x14 0x0bdc9d2d256b3ee9daae347be6f4dc835a467ffe EQUAL")
	if err != nil {
		t.Fatal(err)
	}
	if got := VerifyScriptTest(nil, pub2, ScriptVerifyMinimalData); got != ScriptErrOK {
		t.Fatalf("RIPEMD160: %s", got)
	}
}

func TestEvalSwapAndSize(t *testing.T) {
	pub, err := ParseScriptASM("0 1 SWAP SIZE 0 EQUAL")
	if err != nil {
		t.Fatal(err)
	}
	if got := VerifyScriptTest(nil, pub, 0); got != ScriptErrOK {
		t.Fatalf("SWAP SIZE: %s", got)
	}
}
