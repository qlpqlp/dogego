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

func TestDecodeRejectPayload_minimal(t *testing.T) {
	var b bytes.Buffer
	_ = WriteCompactSize(&b, 7)
	b.WriteString("version")
	b.WriteByte(0x11)
	_ = WriteCompactSize(&b, 4)
	b.WriteString("nope")

	rj, err := DecodeRejectPayload(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if rj.Message != "version" || rj.Code != 0x11 || rj.Reason != "nope" || rj.HashLE != nil {
		t.Fatalf("%+v", rj)
	}
}

func TestDecodeRejectPayload_blockWithHash(t *testing.T) {
	var b bytes.Buffer
	_ = WriteCompactSize(&b, 5)
	b.WriteString("block")
	b.WriteByte(0x10)
	_ = WriteCompactSize(&b, 4)
	b.WriteString("bad!")
	h := make([]byte, 32)
	for i := range h {
		h[i] = byte(i)
	}
	b.Write(h)

	rj, err := DecodeRejectPayload(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if rj.Message != "block" || rj.HashLE == nil {
		t.Fatalf("%+v", rj)
	}
}

func TestDecodeRejectPayload_trailingJunk(t *testing.T) {
	var b bytes.Buffer
	_ = WriteCompactSize(&b, 3)
	b.WriteString("foo")
	b.WriteByte(1)
	_ = WriteCompactSize(&b, 1)
	b.WriteString("x")
	b.WriteByte(99) // junk

	_, err := DecodeRejectPayload(b.Bytes())
	if err == nil {
		t.Fatal("expected error")
	}
}
