// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"testing"

	"dogego/pow"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestExecScanBlocksRequiresFilterIndex(t *testing.T) {
	j := &memJournal{tip: 0, count: 1, hdrs: [][]byte{make([]byte, 80)}}
	arr, _ := json.Marshal([]string{`raw(51)`})
	_, code, msg := execScanBlocks("test", j, nil, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"start"`),
		arr,
	})
	if code != -1 || msg == "" {
		t.Fatalf("want -1, got %d %q", code, msg)
	}
}

func TestExecScanBlocksFindsMatchingBlock(t *testing.T) {
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

	spk := []byte{0x76, 0xa9, 0x14, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 0x88, 0xac}
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: cbTx.TxHash(), PrevIdx: 0, Script: []byte{0x01}, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 8700000000, PkScript: spk}},
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
	fx, _ := store.OpenBlockFilterIndex(dir)
	_ = raw.Put(id0, block0.Bytes())
	_ = raw.Put(id1, block1.Bytes())
	_ = ix.IndexBlock(id0, block0.Bytes())
	_ = ix.IndexBlock(id1, block1.Bytes())
	if err := IndexBasicBlockFilter(fx, id0, block0.Bytes(), nil, raw, ix); err != nil {
		t.Fatal(err)
	}
	if err := IndexBasicBlockFilter(fx, id1, block1.Bytes(), nil, raw, ix); err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 1, best: pow.BlockHashHex(h1[:]), gen: pow.BlockHashHex(h0[:]), count: 2, hdrs: [][]byte{append([]byte(nil), h0[:]...), append([]byte(nil), h1[:]...)}}

	desc := `raw(` + hex.EncodeToString(spk) + `)`
	arr, _ := json.Marshal([]string{desc})
	res, code, msg := execScanBlocks("test", j, raw, ix, fx, nil, []json.RawMessage{
		json.RawMessage(`"start"`),
		arr,
		json.RawMessage(`0`),
		json.RawMessage(`1`),
	})
	if code != 0 {
		t.Fatalf("scanblocks: %d %s", code, msg)
	}
	m := res.(map[string]interface{})
	blocks, ok := m["relevant_blocks"].([]string)
	if !ok || len(blocks) != 1 {
		t.Fatalf("relevant_blocks %#v", m["relevant_blocks"])
	}
	want := pow.LEUint256DisplayHex(id1[:])
	if blocks[0] != want {
		t.Fatalf("got %s want %s", blocks[0], want)
	}
}

func TestExecScanBlocksAbortStatus(t *testing.T) {
	j := &memJournal{tip: 0, count: 1, hdrs: [][]byte{make([]byte, 80)}}
	dir := t.TempDir()
	fx, _ := store.OpenBlockFilterIndex(dir)
	if v, code, _ := execScanBlocks("test", j, nil, nil, fx, nil, []json.RawMessage{json.RawMessage(`"abort"`)}); code != 0 || v != false {
		t.Fatalf("abort: %v %d", v, code)
	}
	if _, code, _ := execScanBlocks("test", j, nil, nil, fx, nil, []json.RawMessage{json.RawMessage(`"status"`)}); code != 0 {
		t.Fatalf("status code %d", code)
	}
}
