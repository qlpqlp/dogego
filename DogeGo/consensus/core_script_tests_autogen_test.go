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

// TestCoreScriptTestsAutogenP2PKWithDogeGoSign shows auto-generated script_tests P2PK layout passes when signed with DogeGo sighash.
func TestCoreScriptTestsAutogenP2PKWithDogeGoSign(t *testing.T) {
	pub, err := ParseScriptASM(scriptTestP2PKPubASM)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := signScriptTestSpend(scriptTestKey0, pub, wire.SigHashAll)
	if err != nil {
		t.Fatal(err)
	}
	if got := VerifyScriptTestSpend(sig, pub, 0); got != ScriptErrOK {
		t.Fatalf("got %s", got)
	}
}
