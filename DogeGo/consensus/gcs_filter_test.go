// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"testing"

	"dogego/wire"
)

func TestBuildBasicGCSFilterEmpty(t *testing.T) {
	var h [32]byte
	h[0] = 0xab
	enc := BuildBasicGCSFilter(h, nil, nil)
	if len(enc) == 0 {
		t.Fatal("expected compact size prefix")
	}
	var r bytes.Buffer
	r.Write(enc)
	n, err := wire.ReadCompactSize(&r)
	if err != nil || n != 0 {
		t.Fatalf("N=%d err=%v", n, err)
	}
}

func TestBuildBasicGCSFilterNonEmpty(t *testing.T) {
	var h [32]byte
	h[1] = 0xcd
	spk := []byte{0x76, 0xa9, 0x14, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 0x88, 0xac}
	enc := BuildBasicGCSFilter(h, [][]byte{spk}, nil)
	if len(enc) < 2 {
		t.Fatal("filter too short")
	}
	if BasicFilterElementCount([][]byte{spk}, nil) != 1 {
		t.Fatal("element count")
	}
}
