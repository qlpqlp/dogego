// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"testing"

	"dogego/primitives"
)

func TestBlockTxDiskOffsets(t *testing.T) {
	tx := Tx{
		Version: 1,
		Vin:     []TxIn{{PrevIdx: 0xffffffff}},
		Vout:    []TxOut{{Value: 50e8, PkScript: []byte{0x00}}},
	}
	var body []byte
	hdr := primitives.BlockHeader{Version: 1, Timestamp: 1}
	h80 := hdr.EncodeWire80()
	body = append(body, h80[:]...)
	// compact size 1 + tx
	var compact bytes.Buffer
	if err := WriteCompactSize(&compact, 1); err != nil {
		t.Fatal(err)
	}
	buf := compact.Bytes()
	txBytes := tx.SerializeForHash()
	body = append(body, buf...)
	body = append(body, txBytes...)

	offsets, err := BlockTxDiskOffsets(body)
	if err != nil {
		t.Fatal(err)
	}
	want := uint32(80 + len(buf)) // header + compact tx count + first tx
	if len(offsets) != 1 || offsets[0] != want {
		t.Fatalf("offsets %v want %d", offsets, want)
	}
}
