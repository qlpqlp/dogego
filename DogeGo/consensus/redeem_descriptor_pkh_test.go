// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/chain"
)

func TestP2SHRedeemDescriptorCLTVPKH(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr, _ := chain.RandomP2PKHAddress(p)
	_, h160, _ := chain.Base58CheckDecode(addr)
	redeem := BuildCLTVP2PKHRedeemScript(42, h160)
	desc, ok := P2SHRedeemDescriptor(redeem, p.PubkeyHashAddrID)
	if !ok || desc != "sh(cltv(42)pkh("+addr+"))" {
		t.Fatalf("desc=%q ok=%v", desc, ok)
	}
}
