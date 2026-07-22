// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire_test

import (
	"bytes"
	"testing"

	"dogego/wire"
)

func TestEncodeGetHeaders(t *testing.T) {
	var loc [32]byte
	loc[0] = 0xab
	var stop [32]byte
	b, err := wire.EncodeGetHeaders(70015, [][32]byte{loc}, stop)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 4+1+32+32 {
		t.Fatalf("short payload %d", len(b))
	}
	if b[4] != 1 {
		t.Fatalf("expected compact size 1, got %d", b[4])
	}
	if !bytes.Equal(b[5:37], loc[:]) {
		t.Fatal("locator mismatch")
	}
}
