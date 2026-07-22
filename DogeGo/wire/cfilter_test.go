// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestDecodeGetCFiltersPayload(t *testing.T) {
	var stop [32]byte
	stop[0] = 0xab
	pl := append([]byte{FilterTypeBasic}, 0x05, 0x00, 0x00, 0x00)
	pl = append(pl, stop[:]...)
	got, err := DecodeGetCFiltersPayload(pl)
	if err != nil {
		t.Fatal(err)
	}
	if got.FilterType != FilterTypeBasic || got.StartHeight != 5 || got.StopHashLE != stop {
		t.Fatalf("decode mismatch %#v", got)
	}
}

func TestEncodeCFilterPayloadLayout(t *testing.T) {
	var id [32]byte
	id[1] = 1
	enc, err := EncodeCFilterPayload(CFilterPayload{
		BlockHashLE: id,
		FilterType:  FilterTypeBasic,
		Filter:      []byte{9, 8, 7},
		NumElements: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) < 32+1+1+3+4 {
		t.Fatalf("short payload %d", len(enc))
	}
	if !bytes.Equal(enc[:32], id[:]) || enc[32] != FilterTypeBasic {
		t.Fatalf("header mismatch %x", enc[:34])
	}
	r := bytes.NewReader(enc[33:])
	n, err := ReadCompactSize(r)
	if err != nil || n != 3 {
		t.Fatalf("filter size %d err %v", n, err)
	}
	skip := make([]byte, n)
	if _, err := r.Read(skip); err != nil {
		t.Fatal(err)
	}
	var ne uint32
	if err := binary.Read(r, binary.LittleEndian, &ne); err != nil || ne != 2 {
		t.Fatalf("num elements %d err %v", ne, err)
	}
}
