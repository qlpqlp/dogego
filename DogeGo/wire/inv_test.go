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

func TestDecodeInvPayload(t *testing.T) {
	var b bytes.Buffer
	_ = wire.WriteCompactSize(&b, 2)
	_ = binary.Write(&b, binary.LittleEndian, uint32(wire.InvTypeBlock))
	var h [32]byte
	h[0] = 0xab
	_, _ = b.Write(h[:])
	_ = binary.Write(&b, binary.LittleEndian, uint32(wire.InvTypeTx))
	h[1] = 0xcd
	_, _ = b.Write(h[:])
	got, err := wire.DecodeInvPayload(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Type != wire.InvTypeBlock || got[1].Type != wire.InvTypeTx {
		t.Fatalf("got %#v", got)
	}
	if got[0].Hash[0] != 0xab || got[1].Hash[1] != 0xcd {
		t.Fatal("hash mismatch")
	}
}

func TestEncodeGetDataRoundTrip(t *testing.T) {
	inv := []wire.InvEntry{
		{Type: wire.InvTypeBlock, Hash: [32]byte{1, 2, 3}},
		{Type: wire.InvTypeTx, Hash: [32]byte{4, 5}},
	}
	pl, err := wire.EncodeGetData(inv)
	if err != nil {
		t.Fatal(err)
	}
	got, err := wire.DecodeInvPayload(pl)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len %d", len(got))
	}
	if got[0].Type != wire.InvTypeBlock || got[1].Type != wire.InvTypeTx {
		t.Fatalf("%#v", got)
	}
	if got[0].Hash[0] != 1 || got[0].Hash[2] != 3 || got[1].Hash[0] != 4 {
		t.Fatal("hash bytes mismatch")
	}
}
