// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"encoding/hex"
	"testing"
)

func TestMasterKeyFingerprintVector1(t *testing.T) {
	seed, err := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	if err != nil {
		t.Fatal(err)
	}
	fp, err := MasterKeyFingerprint(seed)
	if err != nil {
		t.Fatal(err)
	}
	if fp != 0x3442193e {
		t.Fatalf("fingerprint=%#x want 0x3442193e", fp)
	}
}
