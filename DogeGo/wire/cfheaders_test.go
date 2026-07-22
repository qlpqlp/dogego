// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"testing"
)

func TestCFHeadersRoundTrip(t *testing.T) {
	var stop, prev [32]byte
	stop[1] = 1
	prev[2] = 2
	h0, h1 := [32]byte{3}, [32]byte{4}
	enc, err := EncodeCFHeadersPayload(CFHeadersPayload{
		FilterType:           FilterTypeBasic,
		StopHashLE:           stop,
		PreviousFilterHeader: prev,
		FilterHashes:         [][32]byte{h0, h1},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeCFHeadersPayload(enc)
	if err != nil {
		t.Fatal(err)
	}
	if got.FilterType != FilterTypeBasic || got.StopHashLE != stop || got.PreviousFilterHeader != prev {
		t.Fatalf("header fields %#v", got)
	}
	if len(got.FilterHashes) != 2 || got.FilterHashes[0] != h0 || got.FilterHashes[1] != h1 {
		t.Fatalf("hashes %#v", got.FilterHashes)
	}
}

func TestCFCheckptEncode(t *testing.T) {
	var stop [32]byte
	stop[3] = 3
	hdr := [32]byte{9}
	enc, err := EncodeCFCheckptPayload(CFCheckptPayload{
		FilterType:    FilterTypeBasic,
		StopHashLE:    stop,
		FilterHeaders: [][32]byte{hdr},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) < 34 {
		t.Fatalf("short %d", len(enc))
	}
}
