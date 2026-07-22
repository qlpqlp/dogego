// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"
)

func TestExecTruncateToHeightArgs(t *testing.T) {
	_, code, msg := execTruncateToHeight(nil, nil)
	if code != -32602 {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	_, code, msg = execTruncateToHeight(&DataPaths{}, []json.RawMessage{json.RawMessage(`-1`)})
	if code != -8 {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecTruncateToHeightWiring(t *testing.T) {
	var got int64
	paths := &DataPaths{
		TruncateToHeight: func(h int64) error {
			got = h
			return nil
		},
	}
	out, code, msg := execTruncateToHeight(paths, []json.RawMessage{json.RawMessage(`2`)})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if got != 2 {
		t.Fatalf("height %d", got)
	}
	m, ok := out.(map[string]interface{})
	if !ok || m["truncated_to"] != int64(2) {
		t.Fatalf("out %#v", out)
	}
}

func TestExecTruncateToHeightIdempotentSecondCall(t *testing.T) {
	var calls int
	paths := &DataPaths{
		TruncateToHeight: func(h int64) error {
			calls++
			return nil
		},
	}
	for i := 0; i < 2; i++ {
		out, code, msg := execTruncateToHeight(paths, []json.RawMessage{json.RawMessage(`2`)})
		if code != 0 || msg != "" {
			t.Fatalf("pass %d: code=%d msg=%q", i, code, msg)
		}
		if m, ok := out.(map[string]interface{}); !ok || m["truncated_to"] != int64(2) {
			t.Fatalf("pass %d out %#v", i, out)
		}
	}
	if calls != 2 {
		t.Fatalf("truncate calls=%d want 2", calls)
	}
}
