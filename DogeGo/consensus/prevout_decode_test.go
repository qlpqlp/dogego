// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestDecodeDisplayTxidRoundTrip(t *testing.T) {
	var le [32]byte
	for i := range le {
		le[i] = byte(i)
	}
	display := txidDisplayFromLE(le)
	var back [32]byte
	if err := DecodeDisplayTxid(display, &back); err != nil {
		t.Fatal(err)
	}
	if back != le {
		t.Fatalf("round trip failed: %x vs %x", back, le)
	}
}
