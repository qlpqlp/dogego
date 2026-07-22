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

func TestEncodeRejectRoundTrip(t *testing.T) {
	var h [32]byte
	h[0] = 0xab
	pl, err := EncodeReject("block", RejectInvalid, "bad merkle root", &h)
	if err != nil {
		t.Fatal(err)
	}
	rj, err := DecodeRejectPayload(pl)
	if err != nil {
		t.Fatal(err)
	}
	if rj.Message != "block" || rj.Code != RejectInvalid || rj.Reason != "bad merkle root" || rj.HashLE == nil || rj.HashLE[0] != 0xab {
		t.Fatalf("got %+v", rj)
	}
	pl2, err := EncodeReject("version", RejectMalformed, "too old", nil)
	if err != nil {
		t.Fatal(err)
	}
	rj2, err := DecodeRejectPayload(pl2)
	if err != nil || rj2.Message != "version" {
		t.Fatalf("version reject: %v %v", rj2, err)
	}
	_ = bytes.Buffer{}
}
