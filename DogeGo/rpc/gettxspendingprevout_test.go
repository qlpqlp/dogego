// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
	"testing"

	"dogego/mempool"
	"dogego/wire"
)

func TestExecGetTxSpendingPrevout(t *testing.T) {
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5e8, PkScript: []byte{0x51}}},
	}
	rawParent, _ := parent.Serialize()
	pool := mempool.New(100)
	_ = pool.Add(rawParent)
	parentID := mempool.TxIDDisplayHex(parent.TxHash())

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: parent.TxHash(),
			PrevIdx:  0,
			Sequence: 0xfffffffe,
		}},
		Vout: []wire.TxOut{{Value: 4e8, PkScript: []byte{0x52}}},
	}
	rawSpend, _ := spend.Serialize()
	_ = pool.Add(rawSpend)
	spendID := mempool.TxIDDisplayHex(spend.TxHash())

	arr, _ := json.Marshal([]map[string]interface{}{
		{"txid": parentID, "vout": 0},
		{"txid": strings.Repeat("0", 64), "vout": 0},
	})
	res, code, msg := execGetTxSpendingPrevout(pool, []json.RawMessage{arr})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	rows, ok := res.([]interface{})
	if !ok || len(rows) != 2 {
		t.Fatalf("result %#v", res)
	}
	r0 := rows[0].(map[string]interface{})
	if r0["spendingtxid"] != spendID {
		t.Fatalf("spender %#v", r0)
	}
	r1 := rows[1].(map[string]interface{})
	if _, has := r1["spendingtxid"]; has {
		t.Fatalf("unexpected spender %#v", r1)
	}
}
