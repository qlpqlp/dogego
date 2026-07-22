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

func TestExecGetRawMempoolVerboseObject(t *testing.T) {
	pool := mempool.New(10)
	raw := testMinimalCoinbase(t)
	if err := pool.Add(raw); err != nil {
		t.Fatal(err)
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	id := mempool.TxIDDisplayHex(tx.TxHash())

	res, code, msg := execGetRawMempool(pool, nil, nil, nil)
	if code != 0 || msg != "" {
		t.Fatalf("non-verbose code=%d msg=%q", code, msg)
	}
	arr, ok := res.([]string)
	if !ok || len(arr) != 1 || arr[0] != id {
		t.Fatalf("non-verbose %#v", res)
	}

	verbose, code, msg := execGetRawMempool(pool, nil, nil, []json.RawMessage{json.RawMessage(`true`)})
	if code != 0 || msg != "" {
		t.Fatalf("verbose code=%d msg=%q", code, msg)
	}
	m, ok := verbose.(map[string]interface{})
	if !ok {
		t.Fatalf("verbose type %T", verbose)
	}
	if len(m) != 1 {
		t.Fatalf("verbose map %#v", m)
	}
	ent, ok := m[id].(map[string]interface{})
	if !ok || ent["txid"].(string) != id {
		t.Fatalf("entry %#v", m[id])
	}
}
