// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dogego/mempool"
	"dogego/wire"
)

func TestExecGetMempoolEntryAndAncestors(t *testing.T) {
	pool := mempool.New(10)
	parentRaw := testMinimalCoinbase(t)
	if err := pool.Add(parentRaw); err != nil {
		t.Fatal(err)
	}
	parentTx, err := wire.DeserializeTx(parentRaw)
	if err != nil {
		t.Fatal(err)
	}
	child := &wire.Tx{
		Version: 2,
		Vin: []wire.TxIn{{
			PrevHash: parentTx.TxHash(),
			PrevIdx:  0,
			Script:   []byte{0x51},
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1000, PkScript: []byte{0x51}}},
	}
	childRaw, err := child.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Add(childRaw); err != nil {
		t.Fatal(err)
	}
	childTx, err := wire.DeserializeTx(childRaw)
	if err != nil {
		t.Fatal(err)
	}
	childID := mempool.TxIDDisplayHex(childTx.TxHash())

	p0, _ := json.Marshal(childID)
	res, code, msg := execGetMempoolEntry(pool, nil, nil, []json.RawMessage{p0})
	if code != 0 || msg != "" {
		t.Fatalf("entry code=%d msg=%q", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok || m["txid"].(string) != childID {
		t.Fatalf("entry %#v", res)
	}
	depArr, ok := m["depends"].([]interface{})
	if !ok || len(depArr) != 1 || depArr[0].(string) != mempool.TxIDDisplayHex(parentTx.TxHash()) {
		t.Fatalf("depends %#v", m["depends"])
	}
	if _, ok := m["height"]; !ok {
		t.Fatal("missing height field")
	}

	anc, code, msg := execGetMempoolAncestors(pool, nil, nil, []json.RawMessage{p0, json.RawMessage(`false`)})
	if code != 0 {
		t.Fatalf("ancestors code=%d msg=%q", code, msg)
	}
	arr := anc.([]interface{})
	if len(arr) != 1 {
		t.Fatalf("ancestors %#v", anc)
	}

	desc, code, msg := execGetMempoolDescendants(pool, nil, nil, []json.RawMessage{p0, json.RawMessage(`false`)})
	if code != 0 {
		t.Fatalf("descendants code=%d msg=%q", code, msg)
	}
	if len(desc.([]interface{})) != 0 {
		t.Fatalf("leaf descendants %#v", desc)
	}

	parentID := mempool.TxIDDisplayHex(parentTx.TxHash())
	p1, _ := json.Marshal(parentID)
	desc2, code, msg := execGetMempoolDescendants(pool, nil, nil, []json.RawMessage{p1, json.RawMessage(`true`)})
	if code != 0 {
		t.Fatalf("desc2 code=%d msg=%q", code, msg)
	}
	dm, ok := desc2.(map[string]interface{})
	if !ok || len(dm) != 1 {
		t.Fatalf("verbose descendants %#v", desc2)
	}
}

func TestHandlerGetMempoolEntry404(t *testing.T) {
	pool := mempool.New(10)
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, pool, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := `{"jsonrpc":"1.0","id":1,"method":"getmempoolentry","params":["0000000000000000000000000000000000000000000000000000000000000001"]}`
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error *struct {
			Code float64 `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error == nil || int(out.Error.Code) != -5 {
		t.Fatalf("error %#v", out.Error)
	}
}
