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

type mapPrevOut map[[36]byte]PrevOut

func (m mapPrevOut) Lookup(prevHash [32]byte, idx uint32) (PrevOut, bool) {
	o, ok := m[outpointKey(prevHash, idx)]
	return o, ok
}

func TestComputeBlockFeeStats(t *testing.T) {
	parent := [32]byte{9}
	view := mapPrevOut{
		outpointKey(parent, 0): {Value: 2_000_000_000, PkScript: []byte{0x51}},
	}
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_900_000_000, PkScript: []byte{0x51}}},
	}
	pb := &wire.ParsedBlock{Txs: []*wire.Tx{
		{Version: 1, Vin: []wire.TxIn{{PrevIdx: 0xffffffff}}, Vout: []wire.TxOut{{Value: 0, PkScript: []byte{0x51}}}},
		spend,
	}}
	st, ok := ComputeBlockFeeStats(pb, view)
	if !ok || st.TotalFee != 100_000_000 {
		t.Fatalf("ok=%v stats=%+v", ok, st)
	}
	if st.MaxFee != 100_000_000 || st.FeeratePercentiles[4] == 0 {
		t.Fatalf("%+v", st)
	}
}
