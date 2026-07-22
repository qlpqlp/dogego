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
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dogego/chain"
	"dogego/mempool"
	"dogego/pow"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

type memJournal struct {
	tip   int64
	best  string
	gen   string
	count int64
	hdrs  [][]byte // optional: per-height 80-byte headers for ReadHeaderAt tests
}

func (m *memJournal) TipHeight() (int64, error)         { return m.tip, nil }
func (m *memJournal) BestBlockHashHex() (string, error) { return m.best, nil }
func (m *memJournal) GenesisHashHex() (string, error)   { return m.gen, nil }
func (m *memJournal) Count() (int64, error)             { return m.count, nil }

func (m *memJournal) ReadHeaderAt(h int64) ([]byte, error) {
	if len(m.hdrs) == 0 {
		return nil, fmt.Errorf("no headers in mem journal")
	}
	if len(m.hdrs) == 1 {
		return append([]byte(nil), m.hdrs[0]...), nil
	}
	if h < 0 || int(h) >= len(m.hdrs) {
		return nil, fmt.Errorf("height out of range")
	}
	return append([]byte(nil), m.hdrs[h]...), nil
}

func (m *memJournal) HeightByDisplayHash(hex string) (int64, error) {
	hex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hex), "0x"))
	for i, row := range m.hdrs {
		if strings.EqualFold(pow.BlockHashHex(row), hex) {
			return int64(i), nil
		}
	}
	return -1, fmt.Errorf("not found")
}

func TestHandlerGetBlockCount(t *testing.T) {
	j := &memJournal{tip: 42, best: "aa", gen: "bb", count: 43}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"getblockcount"}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result int64 `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result != 42 {
		t.Fatalf("result %d", out.Result)
	}
}

func TestHandlerGetChainTips(t *testing.T) {
	h80 := make([]byte, 80)
	binary.LittleEndian.PutUint32(h80[0:4], 2)
	binary.LittleEndian.PutUint32(h80[76:80], 0x11223344)
	wantHash := pow.BlockHashHex(h80)
	j := &memJournal{tip: 0, best: wantHash, gen: "g", count: 1, hdrs: [][]byte{append([]byte(nil), h80...)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getchaintips","id":1}`)))
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
	arr, ok := out["result"].([]interface{})
	if !ok || len(arr) != 1 {
		t.Fatalf("result %#v", out["result"])
	}
	tip0, ok := arr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("tip element %#v", arr[0])
	}
	if int64(tip0["height"].(float64)) != 0 {
		t.Fatalf("height %#v", tip0["height"])
	}
	if tip0["hash"].(string) != wantHash {
		t.Fatalf("hash got %q want %q", tip0["hash"], wantHash)
	}
	if int(tip0["branchlen"].(float64)) != 0 {
		t.Fatalf("branchlen %#v", tip0["branchlen"])
	}
	if tip0["status"].(string) != "active" {
		t.Fatalf("status %#v", tip0["status"])
	}
	if tip0["forkpoint"] != nil {
		t.Fatalf("forkpoint %#v want null", tip0["forkpoint"])
	}
}

func TestHandlerGetBlockchainInfoMempool(t *testing.T) {
	j := &memJournal{tip: 0, best: "tiphex", gen: "genhex", count: 1}
	mp := mempool.New(10)
	_ = mp.Add([]byte{0x01, 0x02})
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getblockchaininfo"}`)
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
	if out.Result["mempool_txs"].(float64) != 1 {
		t.Fatalf("mempool_txs: %#v", out.Result["mempool_txs"])
	}
}

func TestHandlerGetBlockHeaderByHeight(t *testing.T) {
	var g80 [80]byte
	binary.LittleEndian.PutUint32(g80[0:4], 1)
	binary.LittleEndian.PutUint32(g80[68:72], 100)
	binary.LittleEndian.PutUint32(g80[72:76], 0x1e0ffff0)
	binary.LittleEndian.PutUint32(g80[76:80], 42)
	best := pow.BlockHashHex(g80[:])
	j := &memJournal{tip: 0, best: best, gen: "aa", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getblockheader","params":[0],"id":1}`)
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
	if out.Result["height"].(float64) != 0 {
		t.Fatalf("height %#v", out.Result["height"])
	}
	if out.Result["hash"].(string) != best {
		t.Fatalf("hash got %s want %s", out.Result["hash"], best)
	}
	if out.Result["confirmations"].(float64) != 1 {
		t.Fatalf("confirmations %#v want 1 at genesis tip", out.Result["confirmations"])
	}
	if out.Result["difficulty"].(float64) <= 0 {
		t.Fatalf("difficulty %#v", out.Result["difficulty"])
	}
	if out.Result["mediantime"].(float64) != 100 {
		t.Fatalf("mediantime %#v want 100 (genesis MTP)", out.Result["mediantime"])
	}
}

func TestHandlerGetBlockHeaderMediantimeAncestorWindow(t *testing.T) {
	h0 := make([]byte, 80)
	binary.LittleEndian.PutUint32(h0[68:72], 111)
	binary.LittleEndian.PutUint32(h0[72:76], 0x1e0ffff0)
	h1 := make([]byte, 80)
	binary.LittleEndian.PutUint32(h1[68:72], 222)
	binary.LittleEndian.PutUint32(h1[72:76], 0x1e0ffff0)
	best := pow.BlockHashHex(h1)
	j := &memJournal{tip: 1, best: best, gen: "g", count: 2, hdrs: [][]byte{h0, h1}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getblockheader","params":[1],"id":1}`)))
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
	if out.Result["time"].(float64) != 222 {
		t.Fatalf("time %#v", out.Result["time"])
	}
	if out.Result["mediantime"].(float64) != 111 {
		t.Fatalf("mediantime %#v want 111 (MTP from height 0 only)", out.Result["mediantime"])
	}
	if out.Result["confirmations"].(float64) != 1 {
		t.Fatalf("confirmations %#v want 1 for tip header", out.Result["confirmations"])
	}
}

func TestHandlerGetBlockHeaderConfirmationsDepth(t *testing.T) {
	h0 := make([]byte, 80)
	h1 := make([]byte, 80)
	h2 := make([]byte, 80)
	binary.LittleEndian.PutUint32(h0[72:76], 0x1e0ffff0)
	binary.LittleEndian.PutUint32(h1[72:76], 0x1e0ffff0)
	binary.LittleEndian.PutUint32(h2[72:76], 0x1e0ffff0)
	binary.LittleEndian.PutUint32(h0[76:80], 10)
	binary.LittleEndian.PutUint32(h1[76:80], 11)
	binary.LittleEndian.PutUint32(h2[76:80], 12)
	best := pow.BlockHashHex(h2)
	j := &memJournal{tip: 2, best: best, gen: "g", count: 3, hdrs: [][]byte{h0, h1, h2}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	for _, tc := range []struct {
		height int
		want   float64
	}{
		{0, 3},
		{1, 2},
		{2, 1},
	} {
		body := []byte(fmt.Sprintf(`{"method":"getblockheader","params":[%d],"id":1}`, tc.height))
		res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		var out struct {
			Result map[string]interface{} `json:"result"`
		}
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			res.Body.Close()
			t.Fatal(err)
		}
		res.Body.Close()
		if out.Result["confirmations"].(float64) != tc.want {
			t.Fatalf("height %d confirmations %#v want %v", tc.height, out.Result["confirmations"], tc.want)
		}
	}
}

func TestHandlerGetBlockHeaderHex(t *testing.T) {
	var g80 [80]byte
	binary.LittleEndian.PutUint32(g80[0:4], 1)
	binary.LittleEndian.PutUint32(g80[68:72], 100)
	binary.LittleEndian.PutUint32(g80[72:76], 0x1e0ffff0)
	binary.LittleEndian.PutUint32(g80[76:80], 42)
	best := pow.BlockHashHex(g80[:])
	j := &memJournal{tip: 0, best: best, gen: "aa", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getblockheader","params":[0,false]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Result) != 160 {
		t.Fatalf("want 160 hex chars, got %d", len(out.Result))
	}
}

func TestHandlerGetDifficulty(t *testing.T) {
	var g80 [80]byte
	binary.LittleEndian.PutUint32(g80[72:76], 0x1e0ffff0)
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getdifficulty"}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result float64 `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result <= 0 {
		t.Fatalf("difficulty %f", out.Result)
	}
}

func TestHandlerGetMiningInfo(t *testing.T) {
	var g80 [80]byte
	binary.LittleEndian.PutUint32(g80[72:76], 0x1e0ffff0)
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	mp := mempool.New(3)
	_ = mp.Add([]byte{0x01})
	_ = mp.Add([]byte{0x02})
	_ = mp.Add([]byte{0x03})
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getmininginfo","id":1}`)))
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
	if out.Result["blocks"].(float64) != 0 {
		t.Fatalf("blocks %#v", out.Result["blocks"])
	}
	if out.Result["pooledtx"].(float64) != 3 {
		t.Fatalf("pooledtx %#v", out.Result["pooledtx"])
	}
	if out.Result["networkhashps"].(float64) <= 0 {
		t.Fatalf("networkhashps %#v", out.Result["networkhashps"])
	}
	if out.Result["testnet"].(bool) != true {
		t.Fatalf("testnet %#v", out.Result["testnet"])
	}
	if out.Result["networkactive"].(bool) != true {
		t.Fatalf("networkactive %#v", out.Result["networkactive"])
	}
}

func TestHandlerGetConnectionCountAndNetTotals(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{
		ConnectionCount: func() int { return 1 },
		NetRecv:         func() int64 { return 0 },
		NetSent:         func() int64 { return 0 },
	}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()

	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getconnectioncount","id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var cc map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&cc); err != nil {
		t.Fatal(err)
	}
	if cc["error"] != nil {
		t.Fatalf("%#v", cc["error"])
	}
	if v, ok := cc["result"].(float64); !ok || int(v) != 1 {
		t.Fatalf("getconnectioncount result %#v", cc["result"])
	}

	res2, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getnettotals","id":2}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	var nt map[string]interface{}
	if err := json.NewDecoder(res2.Body).Decode(&nt); err != nil {
		t.Fatal(err)
	}
	rmap, ok := nt["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result %#v", nt["result"])
	}
	if rmap["totalbytesrecv"].(float64) != 0 {
		t.Fatalf("recv %#v", rmap["totalbytesrecv"])
	}
	ut, _ := rmap["uploadtarget"].(map[string]interface{})
	if ut == nil || ut["serve_historical_blocks"].(bool) != true {
		t.Fatalf("uploadtarget %#v", rmap["uploadtarget"])
	}
	if rmap["timemillis"].(float64) <= 0 {
		t.Fatalf("timemillis %#v", rmap["timemillis"])
	}
}

func TestHandlerGetNetTotalsP2PDisabled(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getnettotals","id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	errObj, _ := out["error"].(map[string]interface{})
	if errObj == nil || int(errObj["code"].(float64)) != CodeRPCP2PDisabled {
		t.Fatalf("%#v", out["error"])
	}
}

func TestHandlerGetNetTotalsWithPaths(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{
		NetRecv: func() int64 { return 100 },
		NetSent: func() int64 { return 50 },
	}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getnettotals","id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var nt map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&nt); err != nil {
		t.Fatal(err)
	}
	rmap, ok := nt["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result %#v", nt["result"])
	}
	if rmap["totalbytesrecv"].(float64) != 100 {
		t.Fatalf("recv %#v", rmap["totalbytesrecv"])
	}
	if rmap["totalbytessent"].(float64) != 50 {
		t.Fatalf("sent %#v", rmap["totalbytessent"])
	}
	if note, _ := rmap["dogego_note"].(string); !strings.Contains(note, "aggregate") {
		t.Fatalf("dogego_note %q", note)
	}
}

func TestHandlerStopWithoutShutdown(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"stop","id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["result"] != nil {
		t.Fatalf("expected error, got result %#v", out["result"])
	}
	errObj, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("error %#v", out["error"])
	}
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "not available") {
		t.Fatalf("message %q", msg)
	}
}

func TestHandlerStopWithShutdown(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	var called atomic.Bool
	paths := &DataPaths{Shutdown: func() { called.Store(true) }}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"stop","id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != nil {
		t.Fatalf("error %#v", out["error"])
	}
	if s, _ := out["result"].(string); s != "DogeGo stopping." {
		t.Fatalf("result %q", s)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !called.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !called.Load() {
		t.Fatal("Shutdown callback was not invoked")
	}
}

func TestHandlerStopWithDeprecatedDetachArg(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	var called atomic.Bool
	paths := &DataPaths{Shutdown: func() { called.Store(true) }}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"stop","params":[true],"id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != nil {
		t.Fatalf("error %#v", out["error"])
	}
	deadline := time.Now().Add(2 * time.Second)
	for !called.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !called.Load() {
		t.Fatal("Shutdown callback was not invoked")
	}
}

func TestHandlerStopTooManyArgs(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{Shutdown: func() {}}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"stop","params":[true,false],"id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["result"] != nil {
		t.Fatalf("expected error, got %#v", out["result"])
	}
}

func TestHandlerGetMemoryInfo(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getmemoryinfo","id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	rmap, ok := out["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result %#v", out["result"])
	}
	locked, ok := rmap["locked"].(map[string]interface{})
	if !ok {
		t.Fatalf("locked %#v", rmap["locked"])
	}
	if locked["chunks_used"].(float64) != 0 {
		t.Fatalf("chunks_used %#v", locked["chunks_used"])
	}
}

func TestHandlerGetNetworkInfo(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getnetworkinfo","id":1}`)
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
	if out.Result["protocolversion"].(float64) != float64(chain.ProtocolVersion) {
		t.Fatalf("protocolversion %#v", out.Result["protocolversion"])
	}
	if out.Result["relayfee"].(float64) != out.Result["minrelaytxfee"].(float64) {
		t.Fatalf("relayfee %v minrelaytxfee %v", out.Result["relayfee"], out.Result["minrelaytxfee"])
	}
	if out.Result["localservices"].(string) != "0000000000000001" {
		t.Fatalf("localservices %#v", out.Result["localservices"])
	}
	arr, ok := out.Result["localservicesnames"].([]interface{})
	if !ok || len(arr) != 1 || arr[0].(string) != "NETWORK" {
		t.Fatalf("localservicesnames %#v", out.Result["localservicesnames"])
	}
	sub, ok := out.Result["subversion"].(string)
	if !ok || (!strings.Contains(sub, "MuchFaster") && !strings.Contains(sub, "DogeGo")) {
		t.Fatalf("subversion %q", sub)
	}
	nets, ok := out.Result["networks"].([]interface{})
	if !ok || len(nets) != 3 {
		t.Fatalf("networks %#v", out.Result["networks"])
	}
	var names []string
	for _, row := range nets {
		m, _ := row.(map[string]interface{})
		names = append(names, m["name"].(string))
	}
	if names[0] != "ipv4" || names[1] != "ipv6" || names[2] != "onion" {
		t.Fatalf("network names %v", names)
	}
	onion, _ := nets[2].(map[string]interface{})
	if onion["reachable"].(bool) {
		t.Fatalf("onion should be unreachable %#v", onion)
	}
	if out.Result["connections_onion"].(float64) != 0 || out.Result["connections_unroutable"].(float64) != 0 {
		t.Fatalf("onion/unroutable %#v %#v", out.Result["connections_onion"], out.Result["connections_unroutable"])
	}
	note, _ := out.Result["dogego_status_note"].(string)
	if !strings.Contains(note, "experimental") {
		t.Fatalf("dogego_status_note %q", note)
	}
	w, _ := out.Result["warnings"].(string)
	if strings.Contains(w, "experimental") {
		t.Fatalf("warnings should be chain-only, got %q", w)
	}
}

func TestHandlerGetNetworkInfoLocalP2PPaths(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{
		LocalP2P: func() (int32, string, uint64) {
			return 70004, "/OverrideUA:9/", 13 // 1|4|8
		},
	}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getnetworkinfo","id":1}`)))
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
	if int64(out.Result["protocolversion"].(float64)) != 70004 {
		t.Fatalf("protocolversion %#v", out.Result["protocolversion"])
	}
	if out.Result["subversion"].(string) != "/OverrideUA:9/" {
		t.Fatalf("subversion %#v", out.Result["subversion"])
	}
	if out.Result["localservices"].(string) != "000000000000000d" {
		t.Fatalf("localservices %#v", out.Result["localservices"])
	}
	names, ok := out.Result["localservicesnames"].([]interface{})
	if !ok || len(names) != 3 {
		t.Fatalf("localservicesnames %#v", out.Result["localservicesnames"])
	}
}

func TestHandlerGetBlockchainInfoDataPaths(t *testing.T) {
	j := &memJournal{tip: 0, best: "t", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	layout := &DataPaths{BaseDataDir: "/dogedata", ChainDataDir: "/dogedata/mainnet"}
	srv := httptest.NewServer(Handler("test", j, nil, layout, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getblockchaininfo","id":1}`)
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
	if out.Result["dogego_base_datadir"].(string) != "/dogedata" {
		t.Fatalf("base %#v", out.Result["dogego_base_datadir"])
	}
	if out.Result["dogego_chain_datadir"].(string) != "/dogedata/mainnet" {
		t.Fatalf("chain %#v", out.Result["dogego_chain_datadir"])
	}
}

func TestHandlerGetIndexInfoNoTxIndex(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getindexinfo","id":1}`)))
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
	note, _ := out.Result["dogego_note"].(string)
	if !strings.Contains(note, "not available") {
		t.Fatalf("note %q", note)
	}
	if _, has := out.Result["txindex"]; has {
		t.Fatalf("unexpected txindex %#v", out.Result["txindex"])
	}
	cs, ok := out.Result["coinstatsindex"].(map[string]interface{})
	if !ok || cs["synced"].(bool) != false {
		t.Fatalf("coinstatsindex %#v", out.Result["coinstatsindex"])
	}
}

func TestHandlerGetIndexInfoWithTxIndex(t *testing.T) {
	dir := t.TempDir()
	ix, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	name := strings.Repeat("d", 64)
	path := filepath.Join(ix.RootDir(), name)
	if err := os.WriteFile(path, make([]byte, 36), 0o600); err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 7, best: "a", gen: "b", count: 8, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, ix, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getindexinfo","id":1}`)))
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
	ti, ok := out.Result["txindex"].(map[string]interface{})
	if !ok {
		t.Fatalf("txindex %#v", out.Result["txindex"])
	}
	if int(ti["dogego_tx_files"].(float64)) != 1 {
		t.Fatalf("files %#v", ti["dogego_tx_files"])
	}
	if int64(ti["size_on_disk"].(float64)) != 36 {
		t.Fatalf("size %#v", ti["size_on_disk"])
	}
	if int64(ti["best_block_height"].(float64)) != 7 {
		t.Fatalf("height %#v", ti["best_block_height"])
	}
	cs, ok := out.Result["coinstatsindex"].(map[string]interface{})
	if !ok || cs["synced"].(bool) != false {
		t.Fatalf("coinstatsindex %#v", out.Result["coinstatsindex"])
	}
}

func TestHandlerGetBlockchainInfoSizeOnDisk(t *testing.T) {
	chainDir := filepath.Join(t.TempDir(), "testnet")
	if err := os.MkdirAll(chainDir, 0o700); err != nil {
		t.Fatal(err)
	}
	hdr := make([]byte, 80)
	if err := os.WriteFile(filepath.Join(chainDir, "headers.bin"), hdr, 0o600); err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	raw, h := store.TestMinimalBlock()
	copy(hdr, raw[:80])
	if err := rs.Put(h, raw); err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "t", gen: "g", count: 1, hdrs: [][]byte{hdr}}
	layout := &DataPaths{ChainDataDir: chainDir}
	srv := httptest.NewServer(Handler("test", j, nil, layout, rs, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getblockchaininfo","id":1}`)))
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
	want := int64(80 + len(raw))
	got := int64(out.Result["size_on_disk"].(float64))
	if got != want {
		t.Fatalf("size_on_disk %d want %d", got, want)
	}
}

func TestHandlerGetPeerInfo(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{
		PeerInfo: func() []map[string]interface{} {
			return []map[string]interface{}{
				{"id": 1, "addr": "192.0.2.1:44556", "inbound": false},
			}
		},
	}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getpeerinfo","id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Result) != 1 {
		t.Fatalf("len %d", len(out.Result))
	}
	if out.Result[0]["addr"].(string) != "192.0.2.1:44556" {
		t.Fatalf("addr %#v", out.Result[0]["addr"])
	}
}

func TestHandlerGetPeerInfoEmptyWithoutPaths(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getpeerinfo","id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Result) != 0 {
		t.Fatalf("want empty slice, got %#v", out.Result)
	}
}

func TestHandlerGetBlockchainInfoChainwork(t *testing.T) {
	bits := uint32(0x1e0ffff0)
	mk := func(nonce uint32) []byte {
		var h [80]byte
		binary.LittleEndian.PutUint32(h[72:76], bits)
		binary.LittleEndian.PutUint32(h[76:80], nonce)
		return append([]byte(nil), h[:]...)
	}
	h0 := mk(1)
	h1 := mk(2)
	w, err := pow.BlockProofFromBits(bits)
	if err != nil {
		t.Fatal(err)
	}
	want := pow.ChainworkHex(new(big.Int).Add(new(big.Int).Set(w), w))
	j := &memJournal{tip: 1, best: pow.BlockHashHex(h1), gen: "g", count: 2, hdrs: [][]byte{h0, h1}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getblockchaininfo","id":1}`)))
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
	got, _ := out.Result["chainwork"].(string)
	if got != want {
		t.Fatalf("chainwork got %q want %q", got, want)
	}
}

func TestHandlerUptimeAndGetRPCInfo(t *testing.T) {
	j := &memJournal{tip: 0, best: "t", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{Uptime: func() int64 { return 999 }}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()

	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"uptime","id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var up map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&up); err != nil {
		t.Fatal(err)
	}
	if up["error"] != nil {
		t.Fatalf("uptime error: %#v", up["error"])
	}
	switch v := up["result"].(type) {
	case float64:
		if int64(v) != 999 {
			t.Fatalf("uptime want 999 got %v", v)
		}
	default:
		t.Fatalf("uptime result type %T %#v", v, v)
	}

	res2, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getrpcinfo","id":2}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	var info map[string]interface{}
	if err := json.NewDecoder(res2.Body).Decode(&info); err != nil {
		t.Fatal(err)
	}
	if info["error"] != nil {
		t.Fatalf("getrpcinfo error: %#v", info["error"])
	}
	rmap, ok := info["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("getrpcinfo result type %T", info["result"])
	}
	if n, ok := rmap["dogego_supported_method_n"].(float64); !ok || int(n) < 5 {
		t.Fatalf("dogego_supported_method_n %#v", rmap["dogego_supported_method_n"])
	}
}

func TestHandlerGetBlockchainInfoAuxpowCoreParityFlag(t *testing.T) {
	j := &memJournal{tip: 0, best: "t", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("main", j, nil, nil, nil, nil, nil, false, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getblockchaininfo","id":1}`)))
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
	if out.Result["dogego_auxpow_parent_chain_id_core_parity"] != true {
		t.Fatalf("parity flag %#v", out.Result["dogego_auxpow_parent_chain_id_core_parity"])
	}
}

func TestHandlerGetBlockchainInfoWarningUnverifiedMempool(t *testing.T) {
	j := &memJournal{tip: 0, best: "t", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getblockchaininfo","id":1}`)
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
	note, _ := out.Result["dogego_status_note"].(string)
	if !strings.Contains(note, "unverified") {
		t.Fatalf("dogego_status_note %q", note)
	}
	w, _ := out.Result["warnings"].(string)
	if strings.Contains(w, "unverified") {
		t.Fatalf("warnings should be chain-only, got %q", w)
	}
}

func TestHandlerGetBlockchainInfoNoUnverifiedWarningWhenStrict(t *testing.T) {
	j := &memJournal{tip: 0, best: "t", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, false, nil))
	defer srv.Close()
	body := []byte(`{"method":"getblockchaininfo","id":1}`)
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
	note, _ := out.Result["dogego_status_note"].(string)
	if strings.Contains(note, "unverified") {
		t.Fatalf("dogego_status_note %q", note)
	}
}

func TestHandlerBatchPingHelp(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`[{"jsonrpc":"1.0","method":"ping","id":1},{"method":"help","id":2}]`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var batch []map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&batch); err != nil {
		t.Fatal(err)
	}
	if len(batch) != 2 {
		t.Fatalf("len %d", len(batch))
	}
	if batch[0]["result"] != nil {
		t.Fatalf("ping result %#v", batch[0]["result"])
	}
	help, ok := batch[1]["result"].(string)
	if !ok || !strings.Contains(help, "getblockcount") || !strings.Contains(help, "getnetworkinfo") || !strings.Contains(help, "getnetworkhashps") || !strings.Contains(help, "getpeerinfo") || !strings.Contains(help, "getblock") || !strings.Contains(help, "getblockstats") || !strings.Contains(help, "getindexinfo") || !strings.Contains(help, "getinfo") || !strings.Contains(help, "sendrawtransaction") || !strings.Contains(help, "signrawtransaction") || !strings.Contains(help, "signmessagewithprivkey") || !strings.Contains(help, "testmempoolaccept") || !strings.Contains(help, "stop") || !strings.Contains(help, "getrawmempool") || !strings.Contains(help, "getmempoolentry") || !strings.Contains(help, "getmempoolancestors") || !strings.Contains(help, "getmempooldescendants") || !strings.Contains(help, "getmempoolinfo") || !strings.Contains(help, "getmininginfo") || !strings.Contains(help, "getmemoryinfo") || !strings.Contains(help, "getnettotals") || !strings.Contains(help, "getconnectioncount") || !strings.Contains(help, "uptime") || !strings.Contains(help, "getrpcinfo") || !strings.Contains(help, "getchaintips") || !strings.Contains(help, "verifychain") || !strings.Contains(help, "verifymessage") || !strings.Contains(help, "gettxout") || !strings.Contains(help, "gettxoutproof") || !strings.Contains(help, "gettxoutsetinfo") || !strings.Contains(help, "invalidateblock") || !strings.Contains(help, "verifytxoutproof") || !strings.Contains(help, "addnode") || !strings.Contains(help, "clearbanned") || !strings.Contains(help, "combinerawtransaction") || !strings.Contains(help, "createmultisig") || !strings.Contains(help, "createrawtransaction") || !strings.Contains(help, "decoderawtransaction") || !strings.Contains(help, "decodescript") || !strings.Contains(help, "disconnectnode") || !strings.Contains(help, "echo") || !strings.Contains(help, "echojson") || !strings.Contains(help, "estimatefee") || !strings.Contains(help, "estimatepriority") || !strings.Contains(help, "estimatesmartfee") || !strings.Contains(help, "estimatesmartpriority") || !strings.Contains(help, "getaddednodeinfo") || !strings.Contains(help, "listbanned") || !strings.Contains(help, "ping") || !strings.Contains(help, "preciousblock") || !strings.Contains(help, "prioritisetransaction") || !strings.Contains(help, "pruneblockchain") || !strings.Contains(help, "reconsiderblock") || !strings.Contains(help, "setban") || !strings.Contains(help, "setmaxconnections") || !strings.Contains(help, "setnetworkactive") {
		t.Fatalf("help %#v", batch[1]["result"])
	}
}

func TestHandlerHelpSpecificCommand(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"help","params":["getblockcount"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Result, "block") {
		t.Fatalf("help text %q", out.Result)
	}
}

func TestHandlerHelpUnknownCommand(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"help","params":["nosuchmethod"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result != "help: unknown command: nosuchmethod" {
		t.Fatalf("got %q", out.Result)
	}
}

func testMinimalCoinbase(t *testing.T) []byte {
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

func TestHandlerGetBlockVerbosity1(t *testing.T) {
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
	body := []byte(`{"method":"getblock","params":[0,1],"id":1}`)
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
	if out.Result["height"].(float64) != 0 {
		t.Fatalf("height %#v", out.Result["height"])
	}
	txs, ok := out.Result["tx"].([]interface{})
	if !ok || len(txs) != 1 {
		t.Fatalf("tx list %#v", out.Result["tx"])
	}
	if out.Result["mediantime"].(float64) != out.Result["time"].(float64) {
		t.Fatalf("genesis mediantime %#v time %#v", out.Result["mediantime"], out.Result["time"])
	}
	sz := out.Result["size"].(float64)
	if out.Result["weight"].(float64) != 4*sz {
		t.Fatalf("block weight %#v size %#v want 4*size", out.Result["weight"], sz)
	}
}

func TestHandlerGetBlockMediantimeUsesHeaderChain(t *testing.T) {
	txb0 := testMinimalCoinbase(t)
	tx0, err := wire.ReadTx(bytes.NewReader(txb0))
	if err != nil {
		t.Fatal(err)
	}
	th0 := tx0.TxHash()
	hdr0 := primitives.BlockHeader{
		Version:    1,
		PrevBlock:  [32]byte{},
		MerkleRoot: th0,
		Timestamp:  4000,
		Bits:       0x1e0ffff0,
		Nonce:      101,
	}
	h80_0 := hdr0.EncodeWire80()
	prevID := pow.BlockHashLE(h80_0[:])

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
	_, _ = buf.Write([]byte{0x51, 0xab})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	txb1 := buf.Bytes()
	tx1, err := wire.ReadTx(bytes.NewReader(txb1))
	if err != nil {
		t.Fatal(err)
	}
	th1 := tx1.TxHash()
	hdr1 := primitives.BlockHeader{
		Version:    1,
		PrevBlock:  prevID,
		MerkleRoot: th1,
		Timestamp:  8000,
		Bits:       0x1e0ffff0,
		Nonce:      202,
	}
	h80_1 := hdr1.EncodeWire80()

	var block1 bytes.Buffer
	_, _ = block1.Write(h80_1[:])
	_ = wire.WriteCompactSize(&block1, 1)
	_, _ = block1.Write(txb1)
	raw1 := block1.Bytes()

	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	id1 := pow.BlockHashLE(h80_1[:])
	if err := rs.Put(id1, raw1); err != nil {
		t.Fatal(err)
	}
	best := pow.BlockHashHex(h80_1[:])
	gen := pow.BlockHashHex(h80_0[:])
	j := &memJournal{tip: 1, best: best, gen: gen, count: 2, hdrs: [][]byte{append([]byte(nil), h80_0[:]...), append([]byte(nil), h80_1[:]...)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, rs, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getblock","params":[1,1],"id":1}`)
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
	if out.Result["time"].(float64) != 8000 {
		t.Fatalf("time %#v", out.Result["time"])
	}
	if out.Result["mediantime"].(float64) != 4000 {
		t.Fatalf("mediantime %#v want 4000 (MTP from height 0 only)", out.Result["mediantime"])
	}
	sz := out.Result["size"].(float64)
	if out.Result["weight"].(float64) != 4*sz {
		t.Fatalf("block weight %#v size %#v", out.Result["weight"], sz)
	}
}

func TestHandlerGetBlockVerbosity2(t *testing.T) {
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
	body := []byte(`{"method":"getblock","params":[0,2],"id":1}`)
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
	txArr, ok := out.Result["tx"].([]interface{})
	if !ok || len(txArr) != 1 {
		t.Fatalf("tx arr %#v", out.Result["tx"])
	}
	tx0, ok := txArr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("tx0 type %#v", txArr[0])
	}
	if _, ok := tx0["vin"]; !ok {
		t.Fatalf("missing vin %#v", tx0)
	}
	if tx0["txid"] == nil {
		t.Fatal("missing txid")
	}
	sz := tx0["size"].(float64)
	if tx0["weight"].(float64) != 4*sz {
		t.Fatalf("tx weight %#v size %#v", tx0["weight"], sz)
	}
}

func TestHandlerGetBlockNoStore(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getblock","params":[0],"id":1}`)
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
}

func TestHandlerSendRawTransaction(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	mp := mempool.New(100)
	rawTx := testMinimalCoinbase(t)
	tx, err := wire.DeserializeTx(rawTx)
	if err != nil {
		t.Fatal(err)
	}
	want := txidToRPC(tx.TxHash())
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"sendrawtransaction","params":["` + hex.EncodeToString(rawTx) + `"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result != want {
		t.Fatalf("txid got %s want %s", out.Result, want)
	}
	if mp.Count() != 1 {
		t.Fatalf("mempool count %d", mp.Count())
	}
}

func TestHandlerSendRawTransactionStrictRejectsCoinbase(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	mp := mempool.New(100)
	rawTx := testMinimalCoinbase(t)
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, false, nil))
	defer srv.Close()
	body := []byte(`{"method":"sendrawtransaction","params":["` + hex.EncodeToString(rawTx) + `"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	errObj, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error, got %#v", out)
	}
	if errObj["code"].(float64) != -26 {
		t.Fatalf("code %#v", errObj["code"])
	}
	msg, _ := errObj["message"].(string)
	if msg != "coinbase" {
		t.Fatalf("message %q want coinbase", msg)
	}
	if mp.Count() != 0 {
		t.Fatalf("mempool count %d", mp.Count())
	}
}

func TestHandlerTestMempoolAcceptStrictRejectsCoinbase(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	mp := mempool.New(100)
	rawTx := testMinimalCoinbase(t)
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, false, nil))
	defer srv.Close()
	body := []byte(`{"method":"testmempoolaccept","params":[["` + hex.EncodeToString(rawTx) + `"]],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Result) != 1 {
		t.Fatalf("len %d", len(out.Result))
	}
	if out.Result[0]["allowed"].(bool) {
		t.Fatalf("expected reject %#v", out.Result[0])
	}
	msg, _ := out.Result[0]["reject-reason"].(string)
	if msg != "coinbase" {
		t.Fatalf("reject %q want coinbase", msg)
	}
	if mp.Count() != 0 {
		t.Fatalf("mempool must stay empty got %d", mp.Count())
	}
}

func TestHandlerTestMempoolAcceptUnverifiedAllows(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	mp := mempool.New(100)
	rawTx := testMinimalCoinbase(t)
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"testmempoolaccept","params":["` + hex.EncodeToString(rawTx) + `"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Result) != 1 || !out.Result[0]["allowed"].(bool) {
		t.Fatalf("result %#v", out.Result)
	}
	r0 := out.Result[0]
	if r0["weight"].(float64) != 4*r0["vsize"].(float64) {
		t.Fatalf("weight %#v vsize %#v", r0["weight"], r0["vsize"])
	}
	if mp.Count() != 0 {
		t.Fatal("dry-run must not add")
	}
}

func TestHandlerTestMempoolAcceptDuplicateInPool(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	mp := mempool.New(100)
	rawTx := testMinimalCoinbase(t)
	hexStr := hex.EncodeToString(rawTx)
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	if _, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"sendrawtransaction","params":["`+hexStr+`"],"id":0}`))); err != nil {
		t.Fatal(err)
	}
	if mp.Count() != 1 {
		t.Fatalf("count %d", mp.Count())
	}
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"testmempoolaccept","params":[["`+hexStr+`"]],"id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result[0]["allowed"].(bool) {
		t.Fatal("expected duplicate reject")
	}
	msg, _ := out.Result[0]["reject-reason"].(string)
	if !strings.Contains(msg, "txn-already-in-mempool") {
		t.Fatalf("reject %q", msg)
	}
}

func TestHandlerTestMempoolAcceptPoolFull(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	mp := mempool.New(1)
	a := testMinimalCoinbase(t)
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, int32(2))
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
	b := buf.Bytes()
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	_, _ = http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"sendrawtransaction","params":["`+hex.EncodeToString(a)+`"],"id":0}`)))
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"testmempoolaccept","params":[["`+hex.EncodeToString(b)+`"]],"id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	msg, _ := out.Result[0]["reject-reason"].(string)
	if msg != "mempool full" {
		t.Fatalf("reject %q want mempool full", msg)
	}
}

func TestHandlerSendRawTransactionRelay(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	mp := mempool.New(100)
	rawTx := testMinimalCoinbase(t)
	tx, err := wire.DeserializeTx(rawTx)
	if err != nil {
		t.Fatal(err)
	}
	want := txidToRPC(tx.TxHash())
	var relayed [][]byte
	relay := func(b []byte) error {
		relayed = append(relayed, append([]byte(nil), b...))
		return nil
	}
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, relay, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"sendrawtransaction","params":["` + hex.EncodeToString(rawTx) + `"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result != want {
		t.Fatalf("txid got %s want %s", out.Result, want)
	}
	if len(relayed) != 1 || !bytes.Equal(relayed[0], rawTx) {
		t.Fatalf("relay: got %d chunks, first match=%v", len(relayed), len(relayed) == 1 && bytes.Equal(relayed[0], rawTx))
	}
}

func TestHandlerSendRawTransactionNoPool(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"sendrawtransaction","params":["00"],"id":1}`)
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
}

func TestHandlerSendRawTransactionMempoolFull(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	mp := mempool.New(1)
	if err := mp.Add([]byte{0x01, 0x02, 0x03}); err != nil {
		t.Fatal(err)
	}
	rawTx := testMinimalCoinbase(t)
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"sendrawtransaction","params":["` + hex.EncodeToString(rawTx) + `"],"id":1}`)
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
		t.Fatal("expected error when mempool full")
	}
}

func TestHandlerGetRawMempoolEmpty(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	mp := mempool.New(10)
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getrawmempool","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []string `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result == nil || len(out.Result) != 0 {
		t.Fatalf("got %#v", out.Result)
	}
}

func TestHandlerGetRawMempoolAfterSend(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	mp := mempool.New(10)
	rawTx := testMinimalCoinbase(t)
	tx, err := wire.DeserializeTx(rawTx)
	if err != nil {
		t.Fatal(err)
	}
	wantID := txidToRPC(tx.TxHash())
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	sendBody := []byte(`{"method":"sendrawtransaction","params":["` + hex.EncodeToString(rawTx) + `"],"id":1}`)
	res1, err := http.Post(srv.URL, "application/json", bytes.NewReader(sendBody))
	if err != nil {
		t.Fatal(err)
	}
	res1.Body.Close()
	body := []byte(`{"method":"getrawmempool","params":[false],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []string `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Result) != 1 || out.Result[0] != wantID {
		t.Fatalf("got %#v want [%s]", out.Result, wantID)
	}
}

func TestHandlerGetRawMempoolVerboseNumeric(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	mp := mempool.New(10)
	rawTx := testMinimalCoinbase(t)
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	sendBody := []byte(`{"method":"sendrawtransaction","params":["` + hex.EncodeToString(rawTx) + `"],"id":1}`)
	res0, err := http.Post(srv.URL, "application/json", bytes.NewReader(sendBody))
	if err != nil {
		t.Fatal(err)
	}
	res0.Body.Close()
	body := []byte(`{"method":"getrawmempool","params":[1],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	tx, err := wire.DeserializeTx(rawTx)
	if err != nil {
		t.Fatal(err)
	}
	wantID := txidToRPC(tx.TxHash())
	var out struct {
		Result map[string]map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Result) != 1 {
		t.Fatalf("got %#v", out.Result)
	}
	ent, ok := out.Result[wantID]
	if !ok {
		t.Fatalf("missing key %s in %#v", wantID, out.Result)
	}
	if ent["txid"] == nil || ent["size"] == nil {
		t.Fatalf("entry %#v", ent)
	}
	sz := ent["size"].(float64)
	if ent["weight"].(float64) != 4*sz {
		t.Fatalf("weight %#v size %#v", ent["weight"], sz)
	}
	fees, ok := ent["fees"].(map[string]interface{})
	if !ok || fees["base"].(float64) != 0 {
		t.Fatalf("fees %#v", ent["fees"])
	}
	if ent["bip125-replaceable"].(bool) != false {
		t.Fatalf("bip125 %#v", ent["bip125-replaceable"])
	}
}

func TestHandlerGetMempoolInfo(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	mp := mempool.New(10)
	rawTx := testMinimalCoinbase(t)
	srv := httptest.NewServer(Handler("test", j, mp, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	sendBody := []byte(`{"method":"sendrawtransaction","params":["` + hex.EncodeToString(rawTx) + `"],"id":1}`)
	res0, err := http.Post(srv.URL, "application/json", bytes.NewReader(sendBody))
	if err != nil {
		t.Fatal(err)
	}
	res0.Body.Close()
	body := []byte(`{"method":"getmempoolinfo","params":[],"id":1}`)
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
	if out.Result["size"].(float64) != 1 {
		t.Fatalf("size %#v", out.Result["size"])
	}
	if int(out.Result["bytes"].(float64)) != len(rawTx) {
		t.Fatalf("bytes %#v", out.Result["bytes"])
	}
	if out.Result["total_fee"].(float64) != 0 {
		t.Fatalf("total_fee %#v", out.Result["total_fee"])
	}
	if seq := int64(out.Result["mempool_sequence"].(float64)); seq < 1 {
		t.Fatalf("mempool_sequence %#v", out.Result["mempool_sequence"])
	}
}

func TestHandlerGetRawMempoolNoPool(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getrawmempool","params":[],"id":1}`)
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
}

func TestHandlerEstimatesmartfee_feeFilter(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1}
	const koinuPerKB = 100_000_000 // 1 DOGE/kB
	layout := &DataPaths{FeeFilter: func() uint64 { return koinuPerKB }}
	srv := httptest.NewServer(Handler("test", j, nil, layout, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"estimatesmartfee","params":[12],"id":1}`)
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
	if out.Result == nil {
		t.Fatal("nil result")
	}
	if out.Result["feerate"].(float64) != 1.0 {
		t.Fatalf("feerate %#v", out.Result["feerate"])
	}
	if out.Result["fee_rate"].(float64) != out.Result["feerate"].(float64) {
		t.Fatalf("fee_rate %#v feerate %#v", out.Result["fee_rate"], out.Result["feerate"])
	}
	if out.Result["blocks"].(float64) != 0 {
		t.Fatalf("blocks %#v want 0 when no market data", out.Result["blocks"])
	}
	errs, _ := out.Result["errors"].([]interface{})
	if len(errs) != 1 {
		t.Fatalf("errors %#v", out.Result["errors"])
	}
	m, ok := errs[0].(map[string]interface{})
	if !ok || m["type"] != "INSUFFICIENT_FEE" {
		t.Fatalf("error entry %#v", errs[0])
	}
}

func TestHandlerEstimatefee_noFeeFilter(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"estimatefee","params":[6],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result float64 `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result != -1 {
		t.Fatalf("want -1 got %v", out.Result)
	}
}

func TestHandlerRPCBasicAuth401(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1}
	auth := &RPCAuth{User: "doge", Password: "secret"}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, auth))
	defer srv.Close()
	body := []byte(`{"method":"ping","id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHandlerRPCBasicAuthOK(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1}
	auth := &RPCAuth{User: "doge", Password: "secret"}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, auth))
	defer srv.Close()
	req, err := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader([]byte(`{"method":"ping","id":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("doge", "secret")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
}
