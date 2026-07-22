// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

func TestTestMempoolAcceptAlreadyKnown(t *testing.T) {
	pool := mempool.New(100)
	raw := minimalCoinbaseTxBytes(t)
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	idx, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	rpcTxid := txidToRPC(tx.TxHash())
	var rec [36]byte
	rec[0] = 2
	if err := os.WriteFile(filepath.Join(idx.RootDir(), rpcTxid), rec[:], 0o600); err != nil {
		t.Fatal(err)
	}
	hexStr, _ := json.Marshal(hex.EncodeToString(raw))
	res, code, msg := execTestMempoolAccept(pool, idx, nil, nil, nil, []json.RawMessage{hexStr}, false, chain.RebootTestnet)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	arr := res.([]map[string]interface{})
	if arr[0]["allowed"].(bool) {
		t.Fatalf("allowed %#v", arr[0])
	}
	if arr[0]["reject-reason"].(string) != "txn-already-known" {
		t.Fatalf("reject %#v", arr[0]["reject-reason"])
	}
}

func TestTestMempoolAcceptMempoolFullReason(t *testing.T) {
	pool := mempool.New(1)
	if err := pool.Add(minimalCoinbaseTxBytes(t)); err != nil {
		t.Fatal(err)
	}
	raw := testSpendTxMissingParent(t)
	hexStr, _ := json.Marshal(hex.EncodeToString(raw))
	res, code, msg := execTestMempoolAccept(pool, nil, nil, nil, nil, []json.RawMessage{hexStr}, true, chain.RebootTestnet)
	if code != 0 {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	arr := res.([]map[string]interface{})
	if arr[0]["reject-reason"].(string) != "mempool full" {
		t.Fatalf("reject %#v", arr[0]["reject-reason"])
	}
}

func TestTestMempoolAcceptMissingInputs(t *testing.T) {
	pool := mempool.New(100)
	raw := testSpendTxMissingParent(t)
	hexStr, _ := json.Marshal(hex.EncodeToString(raw))
	res, code, msg := execTestMempoolAccept(pool, nil, nil, nil, nil, []json.RawMessage{hexStr}, false, chain.RebootTestnet)
	if code != 0 {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	arr := res.([]map[string]interface{})
	if arr[0]["reject-reason"].(string) != "Missing inputs" {
		t.Fatalf("reject %#v", arr[0]["reject-reason"])
	}
}
