// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"

	"dogego/wire"
)

func TestExecDecodePsbtMinimal(t *testing.T) {
	tx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{3},
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	var b bytes.Buffer
	b.Write(wire.PsbtMagic()[:])
	writePsbtKV(&b, []byte{wire.PsbtGlobalUnsignedTx}, tx.SerializeForHash())
	b.WriteByte(0)
	b.WriteByte(0)
	b.WriteByte(0)
	hexPSBT := hex.EncodeToString(b.Bytes())
	res, code, msg := execDecodePsbt("test", []json.RawMessage{json.RawMessage(`"` + hexPSBT + `"`)})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok || m["tx"] == nil {
		t.Fatalf("result %#v", res)
	}
	inputs, ok := m["inputs"].([]interface{})
	if !ok || len(inputs) != 1 {
		t.Fatalf("inputs %#v", m["inputs"])
	}
}

func TestExecDecodePsbtBase64(t *testing.T) {
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{4}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2, PkScript: []byte{0x51}}},
	}
	var b bytes.Buffer
	b.Write([]byte{0x70, 0x73, 0x62, 0x74, 0xff})
	writePsbtKV(&b, []byte{wire.PsbtGlobalUnsignedTx}, tx.SerializeForHash())
	b.WriteByte(0)
	b.WriteByte(0)
	b.WriteByte(0)
	b64 := base64.StdEncoding.EncodeToString(b.Bytes())
	res, code, msg := execDecodePsbt("test", []json.RawMessage{json.RawMessage(`"` + b64 + `"`)})
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	if res == nil {
		t.Fatal("nil result")
	}
}

func writePsbtKV(b *bytes.Buffer, key, val []byte) {
	_ = wire.WriteCompactSize(b, uint64(len(key)))
	_, _ = b.Write(key)
	_ = wire.WriteCompactSize(b, uint64(len(val)))
	_, _ = b.Write(val)
}
