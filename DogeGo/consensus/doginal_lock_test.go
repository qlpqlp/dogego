// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package consensus

import "testing"

func TestParseDoginalLockRedeem(t *testing.T) {
	pub := make([]byte, 33)
	pub[0] = 0x02
	for i := 1; i < 33; i++ {
		pub[i] = byte(i)
	}
	redeem := []byte{0x21}
	redeem = append(redeem, pub...)
	redeem = append(redeem, opCheckSigVerify, opDrop, opDrop, opTrue)
	got, drops, ok := ParseDoginalLockRedeem(redeem)
	if !ok || drops != 2 || len(got) != 33 {
		t.Fatalf("parse ok=%v drops=%d len=%d", ok, drops, len(got))
	}
	if !IsDoginalLockRedeem(redeem) {
		t.Fatal("IsDoginalLockRedeem")
	}
	code, err := signingScriptCodeFromRedeemSimple(redeem)
	if err != nil || len(code) != len(redeem) {
		t.Fatalf("signing code: %v len=%d", err, len(code))
	}
}
