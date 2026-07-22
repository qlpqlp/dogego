// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"testing"
)

func TestScriptNumPayloadSamples(t *testing.T) {
	for _, n := range []int64{-128, -127, 127, 128, 2147483648} {
		p := scriptNumPayload(n)
		e := encodeScriptNum(n)
		t.Logf("%d payload=%s enc=%s", n, hex.EncodeToString(p), hex.EncodeToString(e))
	}
}

func TestScriptASMInt128Size(t *testing.T) {
	sig, _ := ParseScriptASM("-128")
	pub, _ := ParseScriptASM("SIZE 2 EQUAL")
	flags := ParseScriptTestFlags("P2SH,STRICTENC") | ScriptVerifyP2SH
	if got := VerifyScriptTestSpend(sig, pub, flags); got != ScriptErrOK {
		t.Fatalf("got %s", got)
	}
}
