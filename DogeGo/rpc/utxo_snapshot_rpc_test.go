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
	"time"

	"dogego/mempool"
	"dogego/store"
)

func TestExecSaveUtxoSnapshot(t *testing.T) {
	dir := t.TempDir()
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(42)
	paths := &DataPaths{ChainDataDir: dir, Utxo: utxo}
	res, code, msg := execSaveUtxoSnapshot(utxo, paths)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok || m["success"] != true || m["height"] != int64(42) {
		t.Fatalf("result=%v", res)
	}
	waitUtxoSnapshotFile(t, store.UtxoSnapshotPath(dir), 42)
	loaded, err := store.LoadUtxoSnapshot(store.UtxoSnapshotPath(dir))
	if err != nil || loaded == nil || loaded.TipHeight() != 42 {
		t.Fatalf("reload tip=%d err=%v", loaded.TipHeight(), err)
	}
}

func TestHandlerSaveUtxoSnapshot(t *testing.T) {
	dir := t.TempDir()
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(99)
	paths := &DataPaths{ChainDataDir: dir, Utxo: utxo}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, mempool.New(10), paths, nil, nil, nil, true, nil))
	defer srv.Close()

	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"saveutxosnapshot","params":[],"id":1}`)))
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
	if out.Result["success"] != true {
		t.Fatalf("%#v", out.Result)
	}
	waitUtxoSnapshotFile(t, store.UtxoSnapshotPath(dir), 99)
}

func waitUtxoSnapshotFile(t *testing.T, path string, wantTip int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tip, err := store.ReadUtxoSnapshotDiskTip(path)
		if err == nil && tip == wantTip {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	t.Fatalf("utxo snapshot not ready at %s (want tip %d)", path, wantTip)
}
