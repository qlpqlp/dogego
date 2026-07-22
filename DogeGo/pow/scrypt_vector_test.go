// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pow

import (
	"encoding/hex"
	"testing"
)

func displayHexToLEBytes(display string) ([]byte, error) {
	b, err := hex.DecodeString(display)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return b, nil
}

func TestScryptCoreVectors(t *testing.T) {
	inputHex := "020000004c1271c211717198227392b029a64a7971931d351b387bb80db027f270411e398a07046f7d4a08dd815412a8712f874a7ebf0507e3878bd24e20a3b73fd750a667d2f451eac7471b00de6659"
	wantDisplay := "00000000002bef4107f882f6115e0b01f348d21195dacd3582aa2dabd7985806"
	in, err := hex.DecodeString(inputHex)
	if err != nil || len(in) != 80 {
		t.Fatalf("input: %v", err)
	}
	want, err := displayHexToLEBytes(wantDisplay)
	if err != nil {
		t.Fatal(err)
	}
	got := scrypt102411256(in)
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("byte %d: got %x want %x", i, got, want)
		}
	}
}
