// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestReadScriptNumPushPushdata2(t *testing.T) {
	// locktime 500000 as 4-byte LE push via OP_PUSHDATA2
	raw := []byte{0x4d, 0x04, 0x00, 0x20, 0xa1, 0x07, 0x00}
	n, next, err := ReadScriptNumPush(raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 500000 || next != len(raw) {
		t.Fatalf("n=%d next=%d", n, next)
	}
}
