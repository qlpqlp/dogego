// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"testing"
)

func TestBlockFilterIndexPutGet(t *testing.T) {
	dir := t.TempDir()
	fx, err := OpenBlockFilterIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	var id [32]byte
	id[0] = 0x42
	enc := []byte{1, 2, 3}
	var hdr [32]byte
	hdr[1] = 0xab
	if err := fx.Put(id, enc, hdr[:]); err != nil {
		t.Fatal(err)
	}
	gotEnc, gotHdr, err := fx.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotEnc) != string(enc) || gotHdr[1] != 0xab {
		t.Fatalf("round-trip mismatch enc=%v hdr=%x", gotEnc, gotHdr)
	}
}
