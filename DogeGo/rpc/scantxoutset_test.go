// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"

	"dogego/chain"
	"dogego/store"
	"dogego/wire"
)

func TestExecScanTxOutSetRaw(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	spk := []byte{0x51}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	utxo := store.NewUtxoCache()
	utxo.ApplyBlock(&wire.ParsedBlock{
		Txs: []*wire.Tx{{
			Version: 1,
			Vin:     []wire.TxIn{{PrevIdx: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 50e8, PkScript: spk}},
		}},
	}, 0)
	desc := `raw(` + "51" + `)`
	arr, _ := json.Marshal([]string{desc})
	res, code, msg := execScanTxOutSet("test", nil, j, nil, nil, utxo, nil, []json.RawMessage{
		json.RawMessage(`"start"`),
		arr,
	})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	m := res.(map[string]interface{})
	if m["success"] != true {
		t.Fatalf("success %#v", m["success"])
	}
	if scanResultUnspentCount(m) != 1 {
		t.Fatalf("unspents %#v", m["unspents"])
	}
	_ = p
}

func TestExecScanTxOutSetPKH(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	h160 := [20]byte{0x11, 0x22, 0x33}
	spk := chain.P2PKHScriptFromPubKeyHash(h160)
	addr := chain.PayToPubKeyHashAddress(spk, p.PubkeyHashAddrID)
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	utxo := store.NewUtxoCache()
	utxo.ApplyBlock(&wire.ParsedBlock{
		Txs: []*wire.Tx{{
			Version: 1,
			Vin:     []wire.TxIn{{PrevIdx: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1e8, PkScript: spk}},
		}},
	}, 0)
	arr, _ := json.Marshal([]string{"pkh(" + addr + ")"})
	res, code, msg := execScanTxOutSet("test", nil, j, nil, nil, utxo, nil, []json.RawMessage{
		json.RawMessage(`"start"`),
		arr,
	})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	m := res.(map[string]interface{})
	if n := scanResultUnspentCount(m); n != 1 {
		t.Fatalf("unspents %#v", m["unspents"])
	}
}

func scanResultUnspentCount(m map[string]interface{}) int {
	switch u := m["unspents"].(type) {
	case []interface{}:
		return len(u)
	case []map[string]interface{}:
		return len(u)
	default:
		return -1
	}
}

func TestExecScanTxOutSetAbortStatus(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	res, code, msg := execScanTxOutSet("test", nil, j, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"abort"`)})
	if code != 0 || res != false {
		t.Fatalf("abort %v %d %s", res, code, msg)
	}
	res, code, msg = execScanTxOutSet("test", nil, j, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"status"`)})
	if code != 0 || res != nil {
		t.Fatalf("status %v %d %s", res, code, msg)
	}
}
