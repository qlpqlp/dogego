// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/mempool"
	"dogego/wire"
)

func TestExecCreatePsbt(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	inp, _ := json.Marshal([]map[string]interface{}{{"txid": strings.Repeat("a", 64), "vout": 0}})
	outObj, _ := json.Marshal(map[string]float64{addr: 1.5})
	res, code, msg := execCreatePsbt("test", nil, nil, nil, []json.RawMessage{inp, outObj})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	b64, ok := res.(string)
	if !ok || b64 == "" {
		t.Fatalf("result %#v", res)
	}
	psbt, err := wire.ParsePSBT(mustDecodeB64(t, b64))
	if err != nil {
		t.Fatal(err)
	}
	if psbt.UnsignedTx.Version != 2 {
		t.Fatalf("version %d", psbt.UnsignedTx.Version)
	}
	if len(psbt.UnsignedTx.Vin) != 1 || psbt.UnsignedTx.Vin[0].Sequence != wire.MaxBIP125RBFSequence {
		t.Fatalf("vin %#v", psbt.UnsignedTx.Vin[0])
	}
}

func TestExecCreatePsbtOutputArray(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr, _ := chain.RandomP2PKHAddress(p)
	outArr, _ := json.Marshal([]map[string]interface{}{{"data": "0102"}, {addr: 1.0}})
	inp, _ := json.Marshal([]interface{}{})
	res, code, msg := execCreatePsbt("test", nil, nil, nil, []json.RawMessage{inp, outArr})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	psbt, err := wire.ParsePSBT(mustDecodeB64(t, res.(string)))
	if err != nil {
		t.Fatal(err)
	}
	if len(psbt.UnsignedTx.Vout) != 2 {
		t.Fatalf("vout %d", len(psbt.UnsignedTx.Vout))
	}
}

func TestExecCreatePsbtWithMempoolUtxo(t *testing.T) {
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5e8, PkScript: []byte{0x51}}},
	}
	rawP, _ := parent.Serialize()
	pool := mempool.New(50)
	_ = pool.Add(rawP)
	parentID := mempool.TxIDDisplayHex(parent.TxHash())

	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr, _ := chain.RandomP2PKHAddress(p)
	inp, _ := json.Marshal([]map[string]interface{}{{"txid": parentID, "vout": 0}})
	outObj, _ := json.Marshal(map[string]float64{addr: 4.0})
	res, code, msg := execCreatePsbt("test", nil, nil, pool, []json.RawMessage{inp, outObj})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	psbt, err := wire.ParsePSBT(mustDecodeB64(t, res.(string)))
	if err != nil {
		t.Fatal(err)
	}
	if !psbt.InputHasUTXO(0) {
		t.Fatal("expected non_witness_utxo from mempool parent")
	}
}

func TestExecConvertToPsbt(t *testing.T) {
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	ser, _ := tx.Serialize()
	hexTx := hex.EncodeToString(ser)
	res, code, msg := execConvertToPsbt([]json.RawMessage{json.RawMessage(`"` + hexTx + `"`)})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	_, err := wire.ParsePSBT(mustDecodeB64(t, res.(string)))
	if err != nil {
		t.Fatal(err)
	}
}

func TestExecConvertToPsbtRejectsScriptSig(t *testing.T) {
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{2}, PrevIdx: 0, Script: []byte{0x01}, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	ser, _ := tx.Serialize()
	hexTx := hex.EncodeToString(ser)
	_, code, msg := execConvertToPsbt([]json.RawMessage{json.RawMessage(`"` + hexTx + `"`)})
	if code == 0 {
		t.Fatal("expected error for scriptSig")
	}
	if msg == "" {
		t.Fatal("expected message")
	}
}

func TestExecConvertToPsbtPermitSigData(t *testing.T) {
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{3}, PrevIdx: 0, Script: []byte{0x01, 0x02}, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	ser, _ := tx.Serialize()
	hexTx := hex.EncodeToString(ser)
	res, code, msg := execConvertToPsbt([]json.RawMessage{
		json.RawMessage(`"` + hexTx + `"`),
		json.RawMessage(`true`),
	})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	psbt, err := wire.ParsePSBT(mustDecodeB64(t, res.(string)))
	if err != nil {
		t.Fatal(err)
	}
	if len(psbt.UnsignedTx.Vin[0].Script) != 0 {
		t.Fatalf("script not stripped %#v", psbt.UnsignedTx.Vin[0].Script)
	}
}
