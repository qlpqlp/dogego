// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestIsValidSignatureEncoding(t *testing.T) {
	// Minimal valid DER + sighash (synthetic).
	sig := []byte{0x30, 0x06, 0x02, 0x01, 0x01, 0x02, 0x01, 0x02, 0x01}
	if !isValidSignatureEncoding(sig) {
		t.Fatal("expected valid")
	}
	if isValidSignatureEncoding([]byte{0x30, 0x05, 0x02, 0x01, 0x01, 0x02, 0x01, 0x02, 0x01}) {
		t.Fatal("length mismatch should fail")
	}
}
