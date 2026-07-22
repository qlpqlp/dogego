// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/secp256k1"

	"dogego/chain"
)

func TestP2SHRedeemDescriptorCLTVMultisig(t *testing.T) {
	pub, _ := secp256k1.NewPrivateKey()
	ms := buildTestMultisigRedeem(1, pub.PubKey().SerializeCompressed())
	redeem := BuildCLTVMultisigRedeemScript(500_000, ms)
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	desc, ok := P2SHRedeemDescriptor(redeem, p.PubkeyHashAddrID)
	if !ok || desc[:8] != "sh(cltv(" {
		t.Fatalf("desc=%q ok=%v", desc, ok)
	}
}

func TestShTimelockMultiDescriptorFromRedeemCSV(t *testing.T) {
	pub, _ := secp256k1.NewPrivateKey()
	ms := buildTestMultisigRedeem(1, pub.PubKey().SerializeCompressed())
	redeem := BuildCSVMultisigRedeemScript(16, ms)
	desc, ok := ShTimelockMultiDescriptorFromRedeem(redeem, opCheckSequenceVerify, "csv")
	if !ok || desc[:7] != "sh(csv(" {
		t.Fatalf("desc=%q ok=%v", desc, ok)
	}
}
