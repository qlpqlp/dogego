// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"testing"
)

func TestReadScriptPushLargeDER(t *testing.T) {
	sig := make([]byte, 72)
	sig[0] = 0x30
	script := append([]byte{0x4c, byte(len(sig))}, sig...)
	script = append(script, 0x21)
	script = append(script, bytes.Repeat([]byte{0x02}, 33)...)
	data, next, err := ReadScriptPush(script, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 72 || next != 74 {
		t.Fatalf("sig len %d next %d", len(data), next)
	}
	pub, next2, err := ReadScriptPush(script, next)
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != 33 || next2 != len(script) {
		t.Fatalf("pub len %d next %d script %d", len(pub), next2, len(script))
	}
}

func TestReadScriptPushDirectAndOP1(t *testing.T) {
	script := []byte{0x51, 0x02, 0xaa, 0xbb}
	d0, n0, err := ReadScriptPush(script, 0)
	if err != nil || len(d0) != 1 || d0[0] != 1 || n0 != 1 {
		t.Fatalf("op1 push %#v %d err %v", d0, n0, err)
	}
	d1, n1, err := ReadScriptPush(script, n0)
	if err != nil || !bytes.Equal(d1, []byte{0xaa, 0xbb}) || n1 != len(script) {
		t.Fatalf("direct push %#v err %v", d1, err)
	}
}
