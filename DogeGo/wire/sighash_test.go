// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"testing"
)

func TestStripCodeSeparatorsPreservesPushdata2(t *testing.T) {
	script := []byte{0x4d, 0x02, 0x00, 0xaa, 0xbb, 0xab, 0x51}
	out := stripCodeSeparatorsSimple(script)
	if len(out) != len(script)-1 {
		t.Fatalf("len %d want %d", len(out), len(script)-1)
	}
	if !bytes.Equal(out[:5], script[:5]) || out[5] != 0x51 {
		t.Fatalf("out=%x", out)
	}
}
