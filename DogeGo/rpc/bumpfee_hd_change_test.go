// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/chain"
	"dogego/wire"
)

func TestWalletChangeVoutIndexScripts(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr1, _ := chain.RandomP2PKHAddress(p)
	addr2, _ := chain.RandomP2PKHAddress(p)
	pk1 := p2pkhScriptForTest(t, p, addr1)
	pk2 := p2pkhScriptForTest(t, p, addr2)
	tx := &wire.Tx{
		Version: 1,
		Vout: []wire.TxOut{
			{Value: 1_000, PkScript: pk1},
			{Value: 5_000_000, PkScript: pk2},
		},
	}
	idx := walletChangeVoutIndexScripts(tx, [][]byte{pk1, pk2})
	if idx != 1 {
		t.Fatalf("change vout %d want 1", idx)
	}
}
