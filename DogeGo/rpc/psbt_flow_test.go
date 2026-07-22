// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	"dogego/mempool"
	"dogego/wire"
)

func TestExecAnalyzeAndFinalizePsbt(t *testing.T) {
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1e8, PkScript: []byte{0x51}}},
	}
	var b bytes.Buffer
	b.Write(wire.PsbtMagic())
	writePsbtKVFlow(&b, []byte{wire.PsbtGlobalUnsignedTx}, tx.SerializeForHash())
	b.WriteByte(0)
	writePsbtKVFlow(&b, []byte{wire.PsbtInFinalScriptSig}, []byte{0x01, 0x02})
	b.WriteByte(0)
	b.WriteByte(0)
	b64 := base64.StdEncoding.EncodeToString(b.Bytes())

	an, code, msg := execAnalyzePsbt([]json.RawMessage{json.RawMessage(`"` + b64 + `"`)})
	if code != 0 {
		t.Fatalf("analyze %d %s", code, msg)
	}
	am := an.(map[string]interface{})
	if am["next"] != "extractor" {
		t.Fatalf("next %#v", am["next"])
	}

	fin, code, msg := execFinalizePsbt([]json.RawMessage{json.RawMessage(`"` + b64 + `"`)})
	if code != 0 {
		t.Fatalf("finalize %d %s", code, msg)
	}
	fm := fin.(map[string]interface{})
	if fm["complete"] != true {
		t.Fatalf("complete %#v", fm["complete"])
	}
	if _, ok := fm["hex"].(string); !ok {
		t.Fatalf("hex %#v", fm["hex"])
	}
}

func TestExecCombinePsbt(t *testing.T) {
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{2}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2, PkScript: []byte{0x51}}},
	}
	mkEmpty := func() string {
		var b bytes.Buffer
		b.Write(wire.PsbtMagic())
		writePsbtKVFlow(&b, []byte{wire.PsbtGlobalUnsignedTx}, tx.SerializeForHash())
		b.WriteByte(0)
		b.WriteByte(0)
		b.WriteByte(0)
		return base64.StdEncoding.EncodeToString(b.Bytes())
	}
	pub := make([]byte, 33)
	pub[0] = 0x02
	mkSig := func() string {
		var b bytes.Buffer
		b.Write(wire.PsbtMagic())
		writePsbtKVFlow(&b, []byte{wire.PsbtGlobalUnsignedTx}, tx.SerializeForHash())
		b.WriteByte(0)
		key := append([]byte{wire.PsbtInPartialSig}, pub...)
		writePsbtKVFlow(&b, key, []byte{0x30, 0x03, 0x01, 0x02, 0x03})
		b.WriteByte(0)
		b.WriteByte(0)
		return base64.StdEncoding.EncodeToString(b.Bytes())
	}
	arr, _ := json.Marshal([]string{mkEmpty(), mkSig()})
	res, code, msg := execCombinePsbt([]json.RawMessage{arr})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	combined, err := wire.ParsePSBT(mustDecodeB64(t, res.(string)))
	if err != nil {
		t.Fatal(err)
	}
	if len(combined.Inputs[0]) != 1 {
		t.Fatalf("inputs %#v", combined.Inputs[0])
	}
}

func TestExecUtxoUpdatePsbtMempool(t *testing.T) {
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5e8, PkScript: []byte{0x51}}},
	}
	rawP, _ := parent.Serialize()
	pool := mempool.New(50)
	_ = pool.Add(rawP)
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
	var b bytes.Buffer
	b.Write(wire.PsbtMagic())
	writePsbtKVFlow(&b, []byte{wire.PsbtGlobalUnsignedTx}, spend.SerializeForHash())
	b.WriteByte(0)
	b.WriteByte(0)
	b.WriteByte(0)
	b64 := base64.StdEncoding.EncodeToString(b.Bytes())

	_ = parentID
	out, code, msg := execUtxoUpdatePsbt(nil, nil, pool, []json.RawMessage{json.RawMessage(`"` + b64 + `"`)})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	updated, err := wire.ParsePSBT(mustDecodeB64(t, out.(string)))
	if err != nil {
		t.Fatal(err)
	}
	if !updated.InputHasUTXO(0) {
		t.Fatal("expected non_witness_utxo")
	}
}

func mustDecodeB64(t *testing.T, s string) []byte {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func writePsbtKVFlow(b *bytes.Buffer, key, val []byte) {
	_ = wire.WriteCompactSize(b, uint64(len(key)))
	_, _ = b.Write(key)
	_ = wire.WriteCompactSize(b, uint64(len(val)))
	_, _ = b.Write(val)
}

func TestExecAnalyzePsbtFee(t *testing.T) {
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{3}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10e8, PkScript: []byte{0x51}}},
	}
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 9e8, PkScript: []byte{0x52}}},
	}
	psbt, err := wire.NewPsbtFromTx(spend)
	if err != nil {
		t.Fatal(err)
	}
	ser, _ := parent.Serialize()
	psbt.SetInputNonWitnessUtxo(0, ser)
	raw, err := psbt.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(raw)
	an, code, msg := execAnalyzePsbt([]json.RawMessage{json.RawMessage(`"` + b64 + `"`)})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	am := an.(map[string]interface{})
	if am["fee"] == nil {
		t.Fatalf("fee %#v", am)
	}
}

func TestExecFinalizePsbtIncomplete(t *testing.T) {
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{5}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	var b bytes.Buffer
	b.Write(wire.PsbtMagic())
	writePsbtKVFlow(&b, []byte{wire.PsbtGlobalUnsignedTx}, tx.SerializeForHash())
	b.WriteByte(0)
	b.WriteByte(0)
	b.WriteByte(0)
	b64 := base64.StdEncoding.EncodeToString(b.Bytes())
	fin, code, msg := execFinalizePsbt([]json.RawMessage{json.RawMessage(`"` + b64 + `"`), json.RawMessage(`false`)})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	fm := fin.(map[string]interface{})
	if fm["complete"] != false {
		t.Fatalf("complete %#v", fm["complete"])
	}
	if _, ok := fm["psbt"].(string); !ok {
		t.Fatalf("psbt %#v", fm)
	}
}
