// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import "testing"

func TestOpcodeNameCoreParity(t *testing.T) {
	tests := []struct {
		op   byte
		want string
	}{
		{0x00, "0"},
		{0x51, "1"},
		{0x60, "16"},
		{0x63, "OP_IF"},
		{0x75, "OP_DROP"},
		{0xb1, "OP_CHECKLOCKTIMEVERIFY"},
		{0xa6, "OP_RIPEMD160"},
		{0xa8, "OP_SHA256"},
	}
	for _, tc := range tests {
		if got := OpcodeName(tc.op); got != tc.want {
			t.Fatalf("op 0x%02x got %q want %q", tc.op, got, tc.want)
		}
	}
}
