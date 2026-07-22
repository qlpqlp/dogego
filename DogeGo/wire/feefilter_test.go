// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"encoding/binary"
	"testing"
)

func TestDecodeFeeFilterPayload(t *testing.T) {
	pl := make([]byte, 8)
	binary.LittleEndian.PutUint64(pl, 1000)
	v, err := DecodeFeeFilterPayload(pl)
	if err != nil || v != 1000 {
		t.Fatalf("%v %v", v, err)
	}
	_, err = DecodeFeeFilterPayload([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("short payload")
	}
	enc := EncodeFeeFilterPayload(123_456)
	got, err := DecodeFeeFilterPayload(enc)
	if err != nil || got != 123_456 {
		t.Fatalf("roundtrip: %d %v", got, err)
	}
}
