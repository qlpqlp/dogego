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
	"os"
	"testing"

	"dogego/chain"
	"dogego/mempool"
	"dogego/wire"
)

func TestHandlerSaveMempool(t *testing.T) {
	mp := mempool.New(100)
	raw := testMinimalCoinbase(t)
	if err := mp.Add(raw); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	paths := &DataPaths{ChainDataDir: dir}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, mp, paths, nil, nil, nil, true, nil))
	defer srv.Close()

	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"savemempool","params":[],"id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result bool `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.Result {
		t.Fatal("expected true")
	}
	if _, err := os.Stat(mempool.PersistPath(dir)); err != nil {
		t.Fatal(err)
	}
}

func TestHandlerLoadMempoolEmpty(t *testing.T) {
	dir := t.TempDir()
	paths := &DataPaths{ChainDataDir: dir}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, mempool.New(10), paths, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"loadmempool","params":[],"id":1}`)))
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
	if int(out.Result["loaded"].(float64)) != 0 {
		t.Fatalf("%#v", out.Result)
	}
}

func TestExecSaveLoadMempoolFeeDeltasRoundTrip(t *testing.T) {
	dir := t.TempDir()
	paths := &DataPaths{ChainDataDir: dir}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	p1 := mempool.New(100)
	raw := testMinimalCoinbase(t)
	if err := p1.Add(raw); err != nil {
		t.Fatal(err)
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	txid := mempool.TxIDDisplayHex(tx.TxHash())
	ghost := "abcd0000abcd0000abcd0000abcd0000abcd0000abcd0000abcd0000abcd0000"
	ok, code, msg := execPrioritiseTransaction(p1, []json.RawMessage{
		json.RawMessage(`"` + txid + `"`),
		json.RawMessage(`0`),
		json.RawMessage(`12000`),
	})
	if code != 0 || !ok {
		t.Fatalf("prioritise in-pool: %d %s", code, msg)
	}
	ok, code, msg = execPrioritiseTransaction(p1, []json.RawMessage{
		json.RawMessage(`"` + ghost + `"`),
		json.RawMessage(`0`),
		json.RawMessage(`3400`),
	})
	if code != 0 || !ok {
		t.Fatalf("prioritise latent: %d %s", code, msg)
	}
	if _, code, msg := execSaveMempool(p1, paths); code != 0 {
		t.Fatalf("save: %d %s", code, msg)
	}
	p2 := mempool.New(100)
	_, code, msg = execLoadMempool(p2, paths, j, nil, nil, chain.RebootTestnet)
	if code != 0 {
		t.Fatalf("load: %d %s", code, msg)
	}
	if p2.FeeDeltaKoinu(txid) != 12000 {
		t.Fatalf("in-pool delta %d", p2.FeeDeltaKoinu(txid))
	}
	if p2.FeeDeltaKoinu(ghost) != 3400 {
		t.Fatalf("latent delta %d", p2.FeeDeltaKoinu(ghost))
	}
}

func TestExecSaveMempoolIdempotentSecondSave(t *testing.T) {
	dir := t.TempDir()
	paths := &DataPaths{ChainDataDir: dir}
	p := mempool.New(50)
	if err := p.Add(testMinimalCoinbase(t)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		res, code, msg := execSaveMempool(p, paths)
		if code != 0 {
			t.Fatalf("save %d: %d %s", i, code, msg)
		}
		if res != true {
			t.Fatalf("save %d: %#v", i, res)
		}
	}
}

func TestExecLoadMempoolIdempotentSecondLoad(t *testing.T) {
	dir := t.TempDir()
	paths := &DataPaths{ChainDataDir: dir}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	p1 := mempool.New(50)
	if err := p1.Add(testMinimalCoinbase(t)); err != nil {
		t.Fatal(err)
	}
	if _, code, msg := execSaveMempool(p1, paths); code != 0 {
		t.Fatalf("save: %d %s", code, msg)
	}
	p := mempool.New(50)
	var first map[string]interface{}
	for i := 0; i < 2; i++ {
		res, code, msg := execLoadMempool(p, paths, j, nil, nil, chain.RebootTestnet)
		if code != 0 {
			t.Fatalf("load %d: %d %s", i, code, msg)
		}
		m, ok := res.(map[string]interface{})
		if !ok {
			t.Fatalf("load %d: %#v", i, res)
		}
		if i == 0 {
			first = m
			continue
		}
		if m["loaded"] != first["loaded"] || m["skipped"] != first["skipped"] {
			t.Fatalf("second load loaded/skipped changed: first=%#v second=%#v", first, m)
		}
	}
}
