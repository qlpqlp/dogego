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

func TestRedeemScriptMetaMultisig(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x90
	_, pub := secp256k1.PrivKeyFromBytes(sec)
	redeem := buildTestMultisigRedeem(1, pub.SerializeCompressed())
	meta := RedeemScriptMeta(redeem)
	if meta["dogego_script_template"] != "multisig" || meta["dogego_multisig_m"] != 1 {
		t.Fatalf("%v", meta)
	}
}
