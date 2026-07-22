// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/json"
	"testing"

	"dogego/pow"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestExecGetBlockFilterRequiresIndex(t *testing.T) {
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, count: 1, hdrs: [][]byte{make([]byte, 80)}}
	h, _ := json.Marshal(float64(0))
	_, code, msg := execGetBlockFilter(j, raw, nil, nil, []json.RawMessage{h})
	if code != -18 || msg == "" {
		t.Fatalf("want -18, got %d %q", code, msg)
	}
}

func TestExecGetBlockFilterBasicTwoBlocks(t *testing.T) {
	txb := minimalCoinbaseTxBytes(t)
	cbTx, err := wire.DeserializeTx(txb)
	if err != nil {
		t.Fatal(err)
	}
	mr0 := wire.BlockMerkleRoot([]*wire.Tx{cbTx})
	hdr0 := primitives.BlockHeader{Version: 1, MerkleRoot: mr0, Timestamp: 1700000000, Bits: 0x1e0ffff0, Nonce: 42}
	h0 := hdr0.EncodeWire80()
	id0 := pow.BlockHashLE(h0[:])
	var block0 bytes.Buffer
	_, _ = block0.Write(h0[:])
	_ = wire.WriteCompactSize(&block0, 1)
	_, _ = block0.Write(txb)

	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: cbTx.TxHash(), PrevIdx: 0, Script: []byte{0x01}, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 8700000000, PkScript: []byte{0x76, 0xa9, 0x14, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 0x88, 0xac}}},
	}
	spendSer, _ := spend.Serialize()
	mr1 := wire.BlockMerkleRoot([]*wire.Tx{spend})
	hdr1 := primitives.BlockHeader{Version: 2, PrevBlock: id0, MerkleRoot: mr1, Timestamp: 1700000100, Bits: 0x1e0ffff0, Nonce: 44}
	h1 := hdr1.EncodeWire80()
	var block1 bytes.Buffer
	_, _ = block1.Write(h1[:])
	_ = wire.WriteCompactSize(&block1, 1)
	_, _ = block1.Write(spendSer)
	id1 := pow.BlockHashLE(h1[:])

	dir := t.TempDir()
	raw, _ := store.OpenRawBlockStore(dir)
	ix, _ := store.OpenTxIndex(dir)
	_ = raw.Put(id0, block0.Bytes())
	_ = raw.Put(id1, block1.Bytes())
	_ = ix.IndexBlock(id0, block0.Bytes())
	_ = ix.IndexBlock(id1, block1.Bytes())
	j := &memJournal{tip: 1, best: pow.BlockHashHex(h1[:]), gen: pow.BlockHashHex(h0[:]), count: 2, hdrs: [][]byte{append([]byte(nil), h0[:]...), append([]byte(nil), h1[:]...)}}

	h, _ := json.Marshal(float64(1))
	res, code, msg := execGetBlockFilter(j, raw, ix, nil, []json.RawMessage{h, json.RawMessage(`"basic"`)})
	if code != 0 {
		t.Fatalf("getblockfilter: %d %s", code, msg)
	}
	m := res.(map[string]interface{})
	if m["filter"] == "" || m["header"] == "" {
		t.Fatalf("result %#v", m)
	}
}

func TestExecGetBlockFilterFromPersistedIndex(t *testing.T) {
	txb := minimalCoinbaseTxBytes(t)
	cbTx, err := wire.DeserializeTx(txb)
	if err != nil {
		t.Fatal(err)
	}
	mr0 := wire.BlockMerkleRoot([]*wire.Tx{cbTx})
	hdr0 := primitives.BlockHeader{Version: 1, MerkleRoot: mr0, Timestamp: 1700000000, Bits: 0x1e0ffff0, Nonce: 42}
	h0 := hdr0.EncodeWire80()
	id0 := pow.BlockHashLE(h0[:])
	var block0 bytes.Buffer
	_, _ = block0.Write(h0[:])
	_ = wire.WriteCompactSize(&block0, 1)
	_, _ = block0.Write(txb)

	dir := t.TempDir()
	raw, _ := store.OpenRawBlockStore(dir)
	ix, _ := store.OpenTxIndex(dir)
	fx, _ := store.OpenBlockFilterIndex(dir)
	_ = raw.Put(id0, block0.Bytes())
	_ = ix.IndexBlock(id0, block0.Bytes())
	if err := IndexBasicBlockFilter(fx, id0, block0.Bytes(), nil, raw, ix); err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, count: 1, hdrs: [][]byte{append([]byte(nil), h0[:]...)}}
	h, _ := json.Marshal(float64(0))
	res, code, msg := execGetBlockFilter(j, raw, ix, fx, []json.RawMessage{h})
	if code != 0 {
		t.Fatalf("getblockfilter: %d %s", code, msg)
	}
	m := res.(map[string]interface{})
	if m["filter"] == "" {
		t.Fatal("empty filter")
	}
	hdrRes, code, msg := execGetBlockFilterHeader(j, raw, ix, fx, []json.RawMessage{h})
	if code != 0 {
		t.Fatalf("getblockfilterheader: %d %s", code, msg)
	}
	hm := hdrRes.(map[string]interface{})
	if hm["header"] != m["header"] {
		t.Fatalf("header mismatch filter=%v header-only=%v", m["header"], hm["header"])
	}
}
