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

func TestCSVOperandFromRedeemMultisig(t *testing.T) {
	pub, _ := secp256k1.NewPrivateKey()
	ms := buildTestMultisigRedeem(1, pub.PubKey().SerializeCompressed())
	redeem := BuildCSVMultisigRedeemScript(3, ms)
	op, ok := CSVOperandFromRedeem(redeem)
	if !ok || op != 3 {
		t.Fatalf("op=%d ok=%v", op, ok)
	}
	if seq := CSVOperandToInputSequence(op); seq != 3 {
		t.Fatalf("seq=%d", seq)
	}
}

func TestCLTVLockTimeFromRedeemMultisig(t *testing.T) {
	pub, _ := secp256k1.NewPrivateKey()
	ms := buildTestMultisigRedeem(1, pub.PubKey().SerializeCompressed())
	redeem := BuildCLTVMultisigRedeemScript(12345, ms)
	lock, ok := CLTVLockTimeFromRedeem(redeem)
	if !ok || lock != 12345 {
		t.Fatalf("lock=%d ok=%v", lock, ok)
	}
}
