// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/mempool"
	"dogego/wire"
)

func TestExecSendRawTransactionWrongArity(t *testing.T) {
	mp := mempool.New(10)
	p0, _ := json.Marshal("00")
	p1, _ := json.Marshal(false)
	p2, _ := json.Marshal(0.01)
	p3, _ := json.Marshal(true)
	_, code, msg := execSendRawTransaction(mp, nil, nil, nil, nil, []json.RawMessage{p0, p1, p2, p3}, nil, true, chain.RebootTestnet)
	if code != -32602 || !strings.Contains(msg, "Wrong number") {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecSendRawTransactionAllowHighFeesType(t *testing.T) {
	mp := mempool.New(10)
	p0, _ := json.Marshal("deadbeef")
	p1, _ := json.Marshal("yes")
	_, code, msg := execSendRawTransaction(mp, nil, nil, nil, nil, []json.RawMessage{p0, p1}, nil, true, chain.RebootTestnet)
	if code != -8 || !strings.Contains(msg, "allowhighfees") {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecSendRawTransactionTXDecodeFailed(t *testing.T) {
	mp := mempool.New(10)
	p0, _ := json.Marshal("not-hex")
	_, code, msg := execSendRawTransaction(mp, nil, nil, nil, nil, []json.RawMessage{p0}, nil, true, chain.RebootTestnet)
	if code != -22 || msg != "TX decode failed" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	p1, _ := json.Marshal("abcd") // valid hex, too short for a tx
	_, code, msg = execSendRawTransaction(mp, nil, nil, nil, nil, []json.RawMessage{p1}, nil, true, chain.RebootTestnet)
	if code != -22 || msg != "TX decode failed" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecSendRawTransactionAllowHighFeesDoesNotBypassConsensus(t *testing.T) {
	mp := mempool.New(100)
	raw := minimalCoinbaseTxBytes(t)
	txHex, _ := json.Marshal(hex.EncodeToString(raw))
	allow, _ := json.Marshal(true)
	_, code, msg := execSendRawTransaction(mp, nil, nil, nil, nil, []json.RawMessage{txHex, allow}, nil, false, chain.RebootTestnet)
	if code != -26 || msg != "coinbase" {
		t.Fatalf("expected policy reject, code=%d msg=%q", code, msg)
	}
	if mp.Count() != 0 {
		t.Fatalf("mempool count %d", mp.Count())
	}
}

func TestExecSendRawTransactionAllowHighFeesTrueAcceptsWhenUnverified(t *testing.T) {
	mp := mempool.New(100)
	raw := minimalCoinbaseTxBytes(t)
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := txidToRPC(tx.TxHash())
	txHex, _ := json.Marshal(hex.EncodeToString(raw))
	allow, _ := json.Marshal(true)
	res, code, msg := execSendRawTransaction(mp, nil, nil, nil, nil, []json.RawMessage{txHex, allow}, nil, true, chain.RebootTestnet)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if res != want {
		t.Fatalf("got %v want %v", res, want)
	}
}
