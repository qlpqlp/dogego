// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestCheckMinimalPush(t *testing.T) {
	if !checkMinimalPush(nil, 0x00) {
		t.Fatal("OP_0")
	}
	if checkMinimalPush(nil, 0x01) {
		t.Fatal("empty should not use OP_PUSHBYTES_1")
	}
	if !checkMinimalPush([]byte{0x05}, 0x55) { // OP_5
		t.Fatal("OP_5 encoding")
	}
	if checkMinimalPush([]byte{0x05}, 0x01) {
		t.Fatal("non-minimal single-byte push")
	}
	// 47-byte push: direct opcode 0x2f vs PUSHDATA1.
	data := make([]byte, 47)
	if !checkMinimalPush(data, 0x2f) {
		t.Fatal("direct push for len 47")
	}
	if checkMinimalPush(data, 0x4c) {
		t.Fatal("PUSHDATA1 should be non-minimal for len 47")
	}
}

func TestCheckScriptMinimalDataRejectsNonMinimal(t *testing.T) {
	// Non-minimal: PUSHDATA1 for 1 byte when OP_1 would work.
	bad := []byte{0x4c, 0x01, 0x01}
	if err := checkScriptMinimalData(bad); err == nil {
		t.Fatal("expected MINIMALDATA rejection")
	}
	// Minimal OP_1.
	if err := checkScriptMinimalData([]byte{0x51}); err != nil {
		t.Fatalf("minimal OP_1: %v", err)
	}
}
