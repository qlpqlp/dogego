// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"

	"dogego/chain"
	"dogego/mempool"
	"dogego/pow"
)

func TestExecCreateAuxBlockLegacyHeightRejected(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr, _ := chain.RandomP2PKHAddress(p)
	raw, _ := json.Marshal(addr)
	_, code, msg := execCreateAuxBlock(j, nil, nil, nil, "testnet", nil, []json.RawMessage{raw})
	if code != -1 || msg == "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecCreateAuxBlockTemplateFields(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{
		tip:   158099,
		count: 158100,
		best:  "x",
		gen:   "y",
		hdrs:  [][]byte{append([]byte(nil), g80[:]...)},
	}
	addr, _ := chain.RandomP2PKHAddress(p)
	raw, _ := json.Marshal(addr)
	res, code, msg := execCreateAuxBlock(j, mempool.New(100), nil, nil, "testnet", &DataPaths{}, []json.RawMessage{raw})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	m := res.(map[string]interface{})
	if _, ok := m["hash"].(string); !ok {
		t.Fatalf("hash %#v", m["hash"])
	}
	if m["chainid"].(int32) != 0x62 {
		t.Fatalf("chainid %#v", m["chainid"])
	}
}
