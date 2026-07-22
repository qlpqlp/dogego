// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"dogego/wire"
)

func TestExecJoinPsbt(t *testing.T) {
	tx1 := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{3}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	tx2 := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{4}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2, PkScript: []byte{0x52}}},
	}
	p1, _ := wire.NewPsbtFromTx(tx1)
	p2, _ := wire.NewPsbtFromTx(tx2)
	b1, _ := p1.Serialize()
	b2, _ := p2.Serialize()
	arr, _ := json.Marshal([]string{
		base64.StdEncoding.EncodeToString(b1),
		base64.StdEncoding.EncodeToString(b2),
	})
	res, code, msg := execJoinPsbt([]json.RawMessage{arr})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	joined, err := wire.ParsePSBT(mustDecodeB64(t, res.(string)))
	if err != nil {
		t.Fatal(err)
	}
	if len(joined.UnsignedTx.Vin) != 2 || len(joined.UnsignedTx.Vout) != 2 {
		t.Fatalf("joined %#v", joined.UnsignedTx)
	}
}
