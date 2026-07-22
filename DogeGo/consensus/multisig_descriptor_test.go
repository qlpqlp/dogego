// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/secp256k1"
)

func TestShMultiDescriptorFromRedeem(t *testing.T) {
	pub, _ := secp256k1.NewPrivateKey()
	redeem := buildTestMultisigRedeem(1, pub.PubKey().SerializeCompressed())
	desc, ok := ShMultiDescriptorFromRedeem(redeem)
	if !ok || desc == "" || desc[:9] != "sh(multi(" {
		t.Fatalf("desc=%q ok=%v", desc, ok)
	}
	_, ok2 := ShMultiDescriptorFromRedeem([]byte{0x00})
	if ok2 {
		t.Fatal("expected false for garbage script")
	}
}
