// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"dogego/wire"
)

func TestGetHeadersRoundTrip(t *testing.T) {
	var loc [32]byte
	loc[0] = 0xab
	var stop [32]byte
	in, err := wire.EncodeGetHeaders(70015, [][32]byte{loc}, stop)
	if err != nil {
		t.Fatal(err)
	}
	got, err := wire.DecodeGetHeaders(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 70015 || len(got.Locator) != 1 || got.Locator[0] != loc {
		t.Fatalf("decode mismatch: %+v", got)
	}
}

func TestHeadersPayloadRoundTrip(t *testing.T) {
	var h80 [80]byte
	binary.LittleEndian.PutUint32(h80[0:4], 1)
	// prev + merkle left zero
	binary.LittleEndian.PutUint32(h80[68:72], 100)
	binary.LittleEndian.PutUint32(h80[72:76], 0x1e0ffff0)
	binary.LittleEndian.PutUint32(h80[76:80], 7)
	var body bytes.Buffer
	_ = wire.WriteCompactSize(&body, 1)
	body.Write(h80[:])
	_ = wire.WriteCompactSize(&body, 0)
	got, err := wire.DecodeHeadersPayload(body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Header80) != 80 {
		t.Fatalf("bad decode len %d %d", len(got), len(got[0].Header80))
	}
	if !bytes.Equal(got[0].Header80, h80[:]) {
		t.Fatal("payload mismatch")
	}
}
