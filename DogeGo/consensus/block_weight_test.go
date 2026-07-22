// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"strings"
	"testing"

	"dogego/wire"
)

func TestCheckBlockWeightRejectsHeavyBlock(t *testing.T) {
	coinbase := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 50, PkScript: []byte{0x51}}},
	}
	big := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: make([]byte, MaxBlockWeight/4+1)}},
	}
	pb := &wire.ParsedBlock{Txs: []*wire.Tx{coinbase, big}}
	err := CheckBlockWeight(pb)
	if err == nil || !strings.Contains(err.Error(), "bad-blk-weight") {
		t.Fatalf("got %v", err)
	}
}
