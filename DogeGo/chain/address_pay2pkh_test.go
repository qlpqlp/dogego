// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import "testing"

func TestPayToPubKeyHashAddress(t *testing.T) {
	// Standard P2PKH script with dummy key hash.
	pk := []byte{0x76, 0xa9, 0x14, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 0x88, 0xac}
	a := PayToPubKeyHashAddress(pk, 0x71)
	if a == "" {
		t.Fatal("expected address")
	}
	if PayToPubKeyHashAddress([]byte{0xaa}, 0x71) != "" {
		t.Fatal("want empty for non-p2pkh")
	}
}
