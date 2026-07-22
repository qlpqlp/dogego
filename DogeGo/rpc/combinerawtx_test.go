// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
	"testing"

	"dogego/chain"
)

func TestExecCombineRawTransactionIdentical(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	txid := strings.Repeat("d", 64)
	inp, _ := json.Marshal([]map[string]interface{}{{"txid": txid, "vout": 0}})
	outObj := map[string]interface{}{addr: 0.12}
	outJSON, _ := json.Marshal(outObj)
	raw1, c1, m1 := execCreateRawTransaction("test", []json.RawMessage{inp, outJSON})
	if c1 != 0 || m1 != "" {
		t.Fatalf("create %d %q", c1, m1)
	}
	arr, _ := json.Marshal([]string{raw1.(string), raw1.(string)})
	merged, c2, m2 := execCombineRawTransaction([]json.RawMessage{arr})
	if c2 != 0 || m2 != "" {
		t.Fatalf("combine %d %q", c2, m2)
	}
	if merged != raw1.(string) {
		t.Fatalf("merged differs")
	}
}

func TestExecCombineRawTransactionConflict(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr, _ := chain.RandomP2PKHAddress(p)
	txid := strings.Repeat("e", 64)
	inp, _ := json.Marshal([]map[string]interface{}{{"txid": txid, "vout": 0}})
	outObj := map[string]interface{}{addr: 0.2}
	outJSON, _ := json.Marshal(outObj)
	a, _, _ := execCreateRawTransaction("test", []json.RawMessage{inp, outJSON})
	b, _, _ := execCreateRawTransaction("test", []json.RawMessage{inp, outJSON})
	if a.(string) != b.(string) {
		t.Fatal("expected identical unsigned hex")
	}
	// Same skeleton, manually break script on second decode path: duplicate hex with altered byte in script region is hard; use two different vout amounts instead.
	out2 := map[string]interface{}{addr: 0.21}
	out2j, _ := json.Marshal(out2)
	other, _, _ := execCreateRawTransaction("test", []json.RawMessage{inp, out2j})
	arr, _ := json.Marshal([]string{a.(string), other.(string)})
	_, code, msg := execCombineRawTransaction([]json.RawMessage{arr})
	if code != -8 || !strings.Contains(msg, "differs") {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}
