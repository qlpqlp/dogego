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

func TestHandlerGetTxOutProofAndVerify(t *testing.T) {
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
	srv := httptest.NewServer(Handler("test", j, mempool.New(1), nil, rs, ix, nil, true, nil))
	defer srv.Close()

	spendID := txidToRPC(spend.TxHash())
	txidsJSON, _ := json.Marshal([]string{spendID})
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"gettxoutproof","params":[` + string(txidsJSON) + `]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result string      `json:"result"`
		Error  interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("gettxoutproof error: %+v", out.Error)
	}
	if len(out.Result) < 160 {
		t.Fatalf("proof too short: len=%d", len(out.Result))
	}

	paramsProof, _ := json.Marshal(out.Result)
	body2 := []byte(`{"jsonrpc":"1.0","id":2,"method":"verifytxoutproof","params":[` + string(paramsProof) + `]}`)
	res2, err := http.Post(srv.URL, "application/json", bytes.NewReader(body2))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	var out2 struct {
		Result []interface{} `json:"result"`
		Error  interface{}   `json:"error"`
	}
	if err := json.NewDecoder(res2.Body).Decode(&out2); err != nil {
		t.Fatal(err)
	}
	if out2.Error != nil {
		t.Fatalf("verify error: %+v", out2.Error)
	}
	if len(out2.Result) != 1 || out2.Result[0] != spendID {
		t.Fatalf("verify result %#v want [%s]", out2.Result, spendID)
	}
}

func TestHandlerVerifyTxOutProofInvalid(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"verifytxoutproof","params":["deadbeef"]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []interface{} `json:"result"`
		Error  interface{}   `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error: %+v", out.Error)
	}
	if out.Result == nil || len(out.Result) != 0 {
		t.Fatalf("want empty array, got %#v", out.Result)
	}
}
