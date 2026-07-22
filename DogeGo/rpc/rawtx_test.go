// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
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

func minimalCoinbaseTxBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, int32(1))
	_ = wire.WriteCompactSize(&buf, 1)
	var zeros [32]byte
	_, _ = buf.Write(zeros[:])
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&buf, 1)
	_, _ = buf.Write([]byte{0x00})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&buf, 1)
	_ = binary.Write(&buf, binary.LittleEndian, int64(8800000000))
	_ = wire.WriteCompactSize(&buf, 2)
	_, _ = buf.Write([]byte{0x51, 0x51})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	return buf.Bytes()
}

func TestDecodeTxHex_minimalCoinbase(t *testing.T) {
	raw := minimalCoinbaseTxBytes(t)
	m, err := DecodeTxHex(hex.EncodeToString(raw), "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m["vin"]; !ok {
		t.Fatal("missing vin")
	}
}

func TestHandlerGetRawTransactionMempool(t *testing.T) {
	j := &memJournal{tip: 10, best: "bb", gen: "aa", count: 11}
	mp := mempool.New(10)
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, int32(1))
	_ = wire.WriteCompactSize(&buf, 1)
	var zeros [32]byte
	_, _ = buf.Write(zeros[:])
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&buf, 1)
	_, _ = buf.Write([]byte{0x00})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&buf, 1)
	_ = binary.Write(&buf, binary.LittleEndian, int64(8800000000))
	_ = wire.WriteCompactSize(&buf, 2)
	_, _ = buf.Write([]byte{0x51, 0x51})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	raw := buf.Bytes()
	if err := mp.Add(raw); err != nil {
		t.Fatal(err)
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	txid := txidToRPC(tx.TxHash())

	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	params0, _ := json.Marshal(txid)
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"getrawtransaction","params":[` + string(params0) + `, true]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result map[string]interface{} `json:"result"`
		Error  interface{}            `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error %+v", out.Error)
	}
	if out.Result == nil {
		t.Fatalf("result %+v", out.Result)
	}
	switch c := out.Result["confirmations"].(type) {
	case float64:
		if c != 0 {
			t.Fatalf("confirmations %v", c)
		}
	case int64:
		if c != 0 {
			t.Fatalf("confirmations %v", c)
		}
	default:
		t.Fatalf("confirmations type %T", c)
	}
	if out.Result["blockhash"] != nil {
		t.Fatalf("mempool tx blockhash %#v want null", out.Result["blockhash"])
	}
}

func TestHandlerDecoderawtransaction(t *testing.T) {
	j := &memJournal{tip: 0, best: "tip", gen: "gen", count: 1}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	hexStr := hex.EncodeToString(minimalCoinbaseTxBytes(t))
	params, _ := json.Marshal(hexStr)
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"decoderawtransaction","params":[` + string(params) + `]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result map[string]interface{} `json:"result"`
		Error  interface{}            `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error %+v", out.Error)
	}
	if out.Result == nil || out.Result["txid"] == nil {
		t.Fatalf("result %+v", out.Result)
	}
	sz := out.Result["size"].(float64)
	if out.Result["weight"].(float64) != 4*sz {
		t.Fatalf("weight %#v size %#v want weight=4*size", out.Result["weight"], sz)
	}
	vouts, ok := out.Result["vout"].([]interface{})
	if !ok || len(vouts) < 1 {
		t.Fatalf("vout %#v", out.Result["vout"])
	}
	spk, ok := vouts[0].(map[string]interface{})["scriptPubKey"].(map[string]interface{})
	if !ok {
		t.Fatalf("scriptPubKey %#v", vouts[0])
	}
	addrs, ok := spk["addresses"].([]interface{})
	if !ok || len(addrs) != 0 {
		t.Fatalf("addresses %#v", spk["addresses"])
	}
}

func TestHandlerGetRawTransactionConfirmedWithHeight(t *testing.T) {
	txb := minimalCoinbaseTxBytes(t)
	rt := bytes.NewReader(txb)
	tx, err := wire.ReadTx(rt)
	if err != nil {
		t.Fatal(err)
	}
	th := tx.TxHash()
	hdr := primitives.BlockHeader{
		Version:    1,
		PrevBlock:  [32]byte{},
		MerkleRoot: th,
		Timestamp:  1700000000,
		Bits:       0x1e0ffff0,
		Nonce:      42,
	}
	var block bytes.Buffer
	h80 := hdr.EncodeWire80()
	_, _ = block.Write(h80[:])
	_ = wire.WriteCompactSize(&block, 1)
	_, _ = block.Write(txb)
	raw := block.Bytes()

	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	id := pow.BlockHashLE(h80[:])
	if err := rs.Put(id, raw); err != nil {
		t.Fatal(err)
	}
	ix, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(id, raw); err != nil {
		t.Fatal(err)
	}

	best := pow.BlockHashHex(h80[:])
	j := &memJournal{tip: 0, best: best, gen: best, count: 1, hdrs: [][]byte{append([]byte(nil), h80[:]...)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, rs, ix, nil, true, nil))
	defer srv.Close()

	wantTxid := txidToRPC(tx.TxHash())
	params, _ := json.Marshal(wantTxid)
	body := []byte(`{"method":"getrawtransaction","params":[` + string(params) + `, true]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result map[string]interface{} `json:"result"`
		Error  interface{}            `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error %+v", out.Error)
	}
	if out.Result["height"].(float64) != 0 {
		t.Fatalf("height %#v", out.Result["height"])
	}
	bh, ok := out.Result["blockhash"].(string)
	if !ok || bh == "" {
		t.Fatalf("blockhash %#v", out.Result["blockhash"])
	}
}
