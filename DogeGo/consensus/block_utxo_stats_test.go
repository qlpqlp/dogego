// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/wire"
)

func TestCountBlockDustOutputs(t *testing.T) {
	pk := make([]byte, 25)
	pk[0] = 0x76
	parent := [32]byte{2}
	pb := &wire.ParsedBlock{Txs: []*wire.Tx{
		{Version: 1, Vin: []wire.TxIn{{PrevIdx: 0xffffffff}}, Vout: []wire.TxOut{{Value: 0, PkScript: pk}}},
		{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: parent, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout: []wire.TxOut{
				{Value: 1, PkScript: pk},
				{Value: HardDustLimitKoinu, PkScript: pk},
			},
		},
	}}
	if n := CountBlockDustOutputs(pb, DefaultStandardPolicy(), MinRelayTxFeePerKB()); n != 1 {
		t.Fatalf("dust=%d", n)
	}
}
