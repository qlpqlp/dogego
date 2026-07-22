// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

func minimalCoinbaseRaw(t *testing.T) []byte {
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

func TestMempoolExistsRPC(t *testing.T) {
	pool := mempool.New(10)
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, pool, nil, nil, nil, nil, true, nil))
	defer srv.Close()

	raw := minimalCoinbaseRaw(t)
	_ = pool.Add(raw)
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	txid := txidToRPC(tx.TxHash())

	body := []byte(`{"method":"mempoolexists","params":["` + txid + `"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
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
		t.Fatal("want true")
	}
}

func TestGetDeploymentInfoRPC(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("main", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getdeploymentinfo","params":[],"id":1}`)
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
	dep, ok := out.Result["deployments"].(map[string]interface{})
	if !ok {
		t.Fatalf("deployments %#v", out.Result)
	}
	if dep["bip34"] == nil || dep["csv"] == nil {
		t.Fatalf("missing deployments %#v", dep)
	}
}

func TestDumpTxOutSetRPC(t *testing.T) {
	dir := t.TempDir()
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	utxo := store.NewUtxoCache()
	utxo.ApplyBlock(&wire.ParsedBlock{
		Txs: []*wire.Tx{{
			Version: 1,
			Vin:     []wire.TxIn{{PrevIdx: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 50e8, PkScript: []byte{0x51}}},
		}},
	}, 0)
	paths := &DataPaths{ChainDataDir: dir, Utxo: utxo}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"dumptxoutset","params":[],"id":1}`)
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
	p, _ := out.Result["path"].(string)
	if p == "" {
		t.Fatalf("result %#v", out.Result)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(p) != dir {
		t.Fatalf("path dir %q want %q", filepath.Dir(p), dir)
	}
}

func TestLoadTxOutSetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	utxo := store.NewUtxoCache()
	utxo.ApplyBlock(&wire.ParsedBlock{
		Txs: []*wire.Tx{{
			Version: 1,
			Vin:     []wire.TxIn{{PrevIdx: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 25e8, PkScript: []byte{0x51}}},
		}},
	}, 0)
	paths := &DataPaths{ChainDataDir: dir, Utxo: utxo}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	dumpBody := []byte(`{"method":"dumptxoutset","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(dumpBody))
	if err != nil {
		t.Fatal(err)
	}
	var dumpOut struct {
		Result map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&dumpOut); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	dumpPath, _ := dumpOut.Result["path"].(string)
	utxo2 := store.NewUtxoCache()
	paths2 := &DataPaths{Utxo: utxo2}
	loadBody, _ := json.Marshal(map[string]interface{}{
		"method": "loadtxoutset",
		"params": []string{dumpPath},
		"id":     2,
	})
	res2, err := http.Post(srv.URL, "application/json", bytes.NewReader(loadBody))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	var loadOut struct {
		Result map[string]interface{} `json:"result"`
		Error  interface{}            `json:"error"`
	}
	if err := json.NewDecoder(res2.Body).Decode(&loadOut); err != nil {
		t.Fatal(err)
	}
	if loadOut.Error != nil {
		t.Fatalf("load error %#v", loadOut.Error)
	}
	if int(loadOut.Result["coins_loaded"].(float64)) != 1 {
		t.Fatalf("coins %#v", loadOut.Result)
	}
	_ = paths2
}
