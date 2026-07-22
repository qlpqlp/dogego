// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"strings"
	"testing"
)

func TestScriptToASMRoundtrip(t *testing.T) {
	asm := "1 IF 1 ELSE 0 ENDIF"
	script, err := ParseScriptASM(asm)
	if err != nil {
		t.Fatal(err)
	}
	got := ScriptToASM(script)
	if got != asm {
		t.Fatalf("roundtrip %q got %q", asm, got)
	}
}

func TestScriptToASMPushData4(t *testing.T) {
	data := make([]byte, 70000)
	for i := range data {
		data[i] = byte(i & 0xff)
	}
	script := appendScriptBytes(nil, data)
	if script[0] != 0x4e {
		t.Fatalf("want OP_PUSHDATA4, got op %02x", script[0])
	}
	asm := ScriptToASM(script)
	if asm == "" || strings.Contains(asm, "OP_PUSHDATA4") {
		t.Fatalf("unexpected asm %q", asm)
	}
	if len(asm) < len(data)*2 {
		t.Fatalf("asm too short for embedded push")
	}
}
