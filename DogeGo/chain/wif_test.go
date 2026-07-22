// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import (
	"testing"
)

func TestEncodeDecodeWIFRoundTrip(t *testing.T) {
	sk := make([]byte, 32)
	sk[31] = 1
	enc, err := EncodeWIF(sk, 193, true)
	if err != nil {
		t.Fatal(err)
	}
	got, comp, err := DecodeWIF(enc, 193)
	if err != nil {
		t.Fatal(err)
	}
	if !comp || len(got) != 32 {
		t.Fatalf("comp=%v len=%d", comp, len(got))
	}
	for i := range sk {
		if got[i] != sk[i] {
			t.Fatalf("byte %d got %x want %x", i, got, sk)
		}
	}
}

func TestDecodeWIFWrongVersion(t *testing.T) {
	sk := make([]byte, 32)
	sk[0] = 3
	enc, err := EncodeWIF(sk, 193, true)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = DecodeWIF(enc, 30)
	if err == nil {
		t.Fatal("expected version error")
	}
}
