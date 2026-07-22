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
	"dogego/pow"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestHandlerGetTxOutConfirmedUnspent(t *testing.T) {
	txb := minimalCoinbaseTxBytes(t)
	cbTx, err := wire.DeserializeTx(txb)
	if err != nil {
		t.Fatal(err)
	}
	mr0 := wire.BlockMerkleRoot([]*wire.Tx{cbTx})
	hdr0 := primitives.BlockHeader{
		Version:    1,
		PrevBlock:  [32]byte{},
		MerkleRoot: mr0,
		Timestamp:  1700000000,
		Bits:       0x1e0ffff0,
		Nonce:      42,
	}
	h0 := hdr0.EncodeWire80()
	id0 := pow.BlockHashLE(h0[:])
	var block0 bytes.Buffer
	_, _ = block0.Write(h0[:])
	_ = wire.WriteCompactSize(&block0, 1)
	_, _ = block0.Write(txb)

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: cbTx.TxHash(),
			PrevIdx:  0,
			Script:   []byte{0x01},
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{
			Value:    8700000000,
			PkScript: []byte{0x51, 0x51},
		}},
		LockTime: 0,
	}
	spendSer, err := spend.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	mr1 := wire.BlockMerkleRoot([]*wire.Tx{spend})
	hdr1 := primitives.BlockHeader{
		Version:    2,
		PrevBlock:  id0,
		MerkleRoot: mr1,
		Timestamp:  1700000100,
		Bits:       0x1e0ffff0,
		Nonce:      44,
	}
	h1 := hdr1.EncodeWire80()
	var block1 bytes.Buffer
	_, _ = block1.Write(h1[:])
	_ = wire.WriteCompactSize(&block1, 1)
	_, _ = block1.Write(spendSer)

	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(id0, block0.Bytes()); err != nil {
		t.Fatal(err)
	}
	id1 := pow.BlockHashLE(h1[:])
	if err := rs.Put(id1, block1.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(id0, block0.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(id1, block1.Bytes()); err != nil {
		t.Fatal(err)
	}

	best := pow.BlockHashHex(h1[:])
	j := &memJournal{tip: 1, best: best, gen: pow.BlockHashHex(h0[:]), count: 2, hdrs: [][]byte{append([]byte(nil), h0[:]...), append([]byte(nil), h1[:]...)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, rs, ix, nil, true, nil))
	defer srv.Close()

	coinID := txidToRPC(cbTx.TxHash())
	params := []byte(`["` + coinID + `", 0, true]`)
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"gettxout","params":` + string(params) + `}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != nil {
		t.Fatalf("error: %#v", out["error"])
	}
	if out["result"] != nil {
		t.Fatalf("coinbase v0 should be spent in block1, want null result got %#v", out["result"])
	}

	spendID := txidToRPC(spend.TxHash())
	params2 := []byte(`["` + spendID + `", 0, true]`)
	body2 := []byte(`{"jsonrpc":"1.0","id":2,"method":"gettxout","params":` + string(params2) + `}`)
	res2, err := http.Post(srv.URL, "application/json", bytes.NewReader(body2))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	var out2 map[string]interface{}
	if err := json.NewDecoder(res2.Body).Decode(&out2); err != nil {
		t.Fatal(err)
	}
	if out2["error"] != nil {
		t.Fatalf("error: %#v", out2["error"])
	}
	m, ok := out2["result"].(map[string]interface{})
	if !ok || m == nil {
		t.Fatalf("want object for spend tx vout, got %#v", out2["result"])
	}
	if m["value"].(float64) <= 0 {
		t.Fatalf("value %#v", m["value"])
	}
}

func TestHandlerGetTxOutMempool(t *testing.T) {
	j := &memJournal{tip: 5, best: "bb", gen: "aa", count: 6}
	mp := mempool.New(10)
	raw := minimalCoinbaseTxBytes(t)
	if err := mp.Add(raw); err != nil {
		t.Fatal(err)
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	txid := txidToRPC(tx.TxHash())
	params := []byte(`["` + txid + `", 0, true]`)
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"gettxout","params":` + string(params) + `}`)
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != nil {
		t.Fatalf("error: %#v", out["error"])
	}
	m, ok := out["result"].(map[string]interface{})
	if !ok || m == nil {
		t.Fatalf("result %#v", out["result"])
	}
	if m["confirmations"].(float64) != 0 {
		t.Fatalf("confirmations %#v", m["confirmations"])
	}
}

func TestMempoolSpendsOutpoint(t *testing.T) {
	p := mempool.New(100)
	cb := minimalCoinbaseTxBytes(t)
	cbTx, err := wire.DeserializeTx(cb)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Add(cb); err != nil {
		t.Fatal(err)
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: cbTx.TxHash(),
			PrevIdx:  0,
			Script:   []byte{0x02},
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1000, PkScript: []byte{0x51}}},
	}
	ser, err := spend.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Add(ser); err != nil {
		t.Fatal(err)
	}
	id := mempool.TxIDDisplayHex(cbTx.TxHash())
	if !p.SpendsOutpoint(id, 0) {
		t.Fatal("expected mempool to spend coinbase outpoint")
	}
}
