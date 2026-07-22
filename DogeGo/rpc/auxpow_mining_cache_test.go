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

func TestCreateAuxBlockReusesCachedTemplate(t *testing.T) {
	globalAuxCache.resetLocked()
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
	pool := mempool.New(100)
	addr, _ := chain.RandomP2PKHAddress(p)
	raw, _ := json.Marshal(addr)
	res1, code, msg := execCreateAuxBlock(j, pool, nil, nil, "testnet", &DataPaths{}, []json.RawMessage{raw})
	if code != 0 {
		t.Fatalf("first: code=%d msg=%q", code, msg)
	}
	h1 := res1.(map[string]interface{})["hash"].(string)
	res2, code, msg := execCreateAuxBlock(j, pool, nil, nil, "testnet", &DataPaths{}, []json.RawMessage{raw})
	if code != 0 {
		t.Fatalf("second: code=%d msg=%q", code, msg)
	}
	h2 := res2.(map[string]interface{})["hash"].(string)
	if h1 != h2 {
		t.Fatalf("expected cached template hash %q got %q", h1, h2)
	}
}
