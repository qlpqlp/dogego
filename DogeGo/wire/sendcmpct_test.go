// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import "testing"

func TestSendCmpctRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		announce bool
		version  uint64
	}{
		{false, 1},
		{false, 2},
		{true, 1},
	} {
		b, err := EncodeSendCmpct(tc.announce, tc.version)
		if err != nil || len(b) != 9 {
			t.Fatalf("encode: %v len=%d", err, len(b))
		}
		got, err := DecodeSendCmpct(b)
		if err != nil {
			t.Fatal(err)
		}
		if got.Announce != tc.announce || got.Version != tc.version {
			t.Fatalf("got %+v want announce=%v version=%d", got, tc.announce, tc.version)
		}
	}
}

func TestDefaultSendCmpctDeclineMatchesCore(t *testing.T) {
	b, err := DefaultSendCmpctDecline()
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeSendCmpct(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Announce || got.Version != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestDecodeSendCmpctRejectsWrongLength(t *testing.T) {
	if _, err := DecodeSendCmpct([]byte{0}); err == nil {
		t.Fatal("expected error for 1-byte legacy payload")
	}
}
