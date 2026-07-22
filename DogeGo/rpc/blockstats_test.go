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
	"strings"
	"testing"

	"dogego/pow"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestHandlerGetBlockStatsCoinbaseOnly(t *testing.T) {
	txb := testMinimalCoinbase(t)
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
		Timestamp:  1747000000,
		Bits:       0x1e0ffff0,
		Nonce:      2139303,
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
	best := pow.BlockHashHex(h80[:])
	j := &memJournal{tip: 0, best: best, gen: best, count: 1, hdrs: [][]byte{append([]byte(nil), h80[:]...)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, rs, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getblockstats","params":[0],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result["txs"].(float64) != 1 {
		t.Fatalf("txs %#v", out.Result["txs"])
	}
	if out.Result["total_size"].(float64) != 0 {
		t.Fatalf("total_size %#v", out.Result["total_size"])
	}
	if out.Result["subsidy"].(float64) != 8800000000 {
		t.Fatalf("subsidy %#v", out.Result["subsidy"])
	}
	if out.Result["mintxsize"].(float64) != 0 {
		t.Fatalf("mintxsize %#v", out.Result["mintxsize"])
	}
	note, ok := out.Result["dogego_note"].(string)
	if !ok || !strings.Contains(note, "totalfee") {
		t.Fatalf("dogego_note %#v", out.Result["dogego_note"])
	}
}

func TestHandlerGetBlockStatsFilteredNoNote(t *testing.T) {
	txb := testMinimalCoinbase(t)
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
		Timestamp:  1747000000,
		Bits:       0x1e0ffff0,
		Nonce:      2139303,
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
	best := pow.BlockHashHex(h80[:])
	j := &memJournal{tip: 0, best: best, gen: best, count: 1, hdrs: [][]byte{append([]byte(nil), h80[:]...)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, rs, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getblockstats","params":[0,["height","txs"]],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out.Result["dogego_note"]; ok {
		t.Fatal("expected no dogego_note when stats filter is used")
	}
	if len(out.Result) != 2 {
		t.Fatalf("want 2 keys got %d %#v", len(out.Result), out.Result)
	}
}

func TestHandlerGetBlockStatsInvalidStatName(t *testing.T) {
	txb := testMinimalCoinbase(t)
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
		Timestamp:  1747000000,
		Bits:       0x1e0ffff0,
		Nonce:      2139303,
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
	best := pow.BlockHashHex(h80[:])
	j := &memJournal{tip: 0, best: best, gen: best, count: 1, hdrs: [][]byte{append([]byte(nil), h80[:]...)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, rs, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getblockstats","params":[0,["notastat"]],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error == nil {
		t.Fatal("expected error")
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerGetBlockStatsTooManyParams(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getblockstats","params":[0,[],1],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -32602 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerPruneBlockchainNoRawStore(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"pruneblockchain","params":[1],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -1 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerPruneBlockchainWrongArity(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"pruneblockchain","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -32602 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}
