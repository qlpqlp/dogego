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
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

func testSpendTxMissingParent(t *testing.T) []byte {
	t.Helper()
	pkScript := make([]byte, 25)
	pkScript[0], pkScript[1], pkScript[2] = 0x76, 0xa9, 0x14
	pkScript[23], pkScript[24] = 0x88, 0xac
	tx := &wire.Tx{
		Version: 2,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{0xaa},
			PrevIdx:  0,
			Sequence: 0xffffffff,
			Script:   bytes.Repeat([]byte{0x51}, 72),
		}},
		Vout: []wire.TxOut{{Value: 50_000_000, PkScript: pkScript}},
	}
	raw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 82 {
		t.Fatalf("tx len %d", len(raw))
	}
	return raw
}

func TestExecSendRawTransactionMissingInputs(t *testing.T) {
	pool := mempool.New(100)
	raw := testSpendTxMissingParent(t)
	txHex, _ := json.Marshal(hex.EncodeToString(raw))
	_, code, msg := execSendRawTransaction(pool, nil, nil, nil, nil, []json.RawMessage{txHex}, nil, false, chain.RebootTestnet)
	if code != -25 || msg != "Missing inputs" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if pool.Count() != 0 {
		t.Fatalf("mempool count %d", pool.Count())
	}
}

func TestExecSendRawTransactionAlreadyInMempoolRelays(t *testing.T) {
	pool := mempool.New(100)
	raw := minimalCoinbaseTxBytes(t)
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := txidToRPC(tx.TxHash())
	txHex, _ := json.Marshal(hex.EncodeToString(raw))
	_, code, msg := execSendRawTransaction(pool, nil, nil, nil, nil, []json.RawMessage{txHex}, nil, true, chain.RebootTestnet)
	if code != 0 {
		t.Fatalf("first: code=%d msg=%q", code, msg)
	}
	var relayed int
	_, code, msg = execSendRawTransaction(pool, nil, nil, nil, nil, []json.RawMessage{txHex}, func([]byte) error {
		relayed++
		return nil
	}, true, chain.RebootTestnet)
	if code != 0 || msg != "" {
		t.Fatalf("second: code=%d msg=%q", code, msg)
	}
	if relayed != 1 {
		t.Fatalf("relayed=%d", relayed)
	}
	if pool.Count() != 1 {
		t.Fatalf("count=%d", pool.Count())
	}
	_ = want
}

func TestExecSendRawTransactionAlreadyInChain(t *testing.T) {
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
	binary.LittleEndian.PutUint32(rec[32:], 0)
	if err := os.WriteFile(filepath.Join(idx.RootDir(), rpcTxid), rec[:], 0o600); err != nil {
		t.Fatal(err)
	}
	txHex, _ := json.Marshal(hex.EncodeToString(raw))
	_, code, msg := execSendRawTransaction(pool, idx, nil, nil, nil, []json.RawMessage{txHex}, nil, true, chain.RebootTestnet)
	if code != -27 || msg != "transaction already in block chain" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestAcceptMempoolTxRPCOrphanMapsToMissingInputs(t *testing.T) {
	pool := mempool.New(100)
	orphans := mempool.NewOrphanPool(10)
	raw := testSpendTxMissingParent(t)
	child, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{OrphanPool: orphans}
	adm := consensus.MempoolAdmission{View: consensus.AdmissionPrevOutView(pool, nil, nil)}
	err = acceptMempoolTxRPC(raw, child, pool, paths, adm)
	if err == nil || err.Error() == "" {
		t.Fatalf("err=%v", err)
	}
	if !consensus.IsMissingInputsErr(err) {
		t.Fatalf("want missing inputs, got %v", err)
	}
	if orphans.Count() != 1 {
		t.Fatalf("orphans=%d", orphans.Count())
	}
}
