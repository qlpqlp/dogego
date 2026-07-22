// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"

	"dogego/mempool"
	"dogego/wire"
)

func TestExecPrioritiseTransactionLatentGetMempoolEntry(t *testing.T) {
	p := mempool.New(100)
	raw := minimalPrioritiseTestRaw(t)
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	txid := mempool.TxIDDisplayHex(tx.TxHash())
	ok, code, msg := execPrioritiseTransaction(p, []json.RawMessage{
		json.RawMessage(`"` + txid + `"`),
		json.RawMessage(`0.0`),
		json.RawMessage(`50000`),
	})
	if code != 0 || !ok {
		t.Fatalf("latent prioritise: code=%d msg=%s", code, msg)
	}
	if err := p.Add(raw); err != nil {
		t.Fatal(err)
	}
	entry, code, msg := execGetMempoolEntry(p, nil, nil, []json.RawMessage{json.RawMessage(`"` + txid + `"`)})
	if code != 0 {
		t.Fatalf("getmempoolentry: code=%d msg=%s", code, msg)
	}
	m := entry.(map[string]interface{})
	if m["modifiedfee"].(float64) <= m["fee"].(float64) {
		t.Fatalf("modifiedfee=%v fee=%v", m["modifiedfee"], m["fee"])
	}
}
