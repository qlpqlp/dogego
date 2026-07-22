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

func TestExecUpgradeTxIndex_EmptyIndex(t *testing.T) {
	dir := t.TempDir()
	paths := &DataPaths{ChainDataDir: dir}
	res, code, msg := execUpgradeTxIndex(paths, nil)
	if code != 0 {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("result %#v", res)
	}
	switch v := m["upgraded"].(type) {
	case int:
		if v != 0 {
			t.Fatalf("upgraded %d", v)
		}
	case float64:
		if int(v) != 0 {
			t.Fatalf("upgraded %v", v)
		}
	default:
		t.Fatalf("upgraded type %T", m["upgraded"])
	}
}

func TestExecUpgradeTxIndex_MaxFilesParam(t *testing.T) {
	dir := t.TempDir()
	paths := &DataPaths{ChainDataDir: dir}
	p, _ := json.Marshal(0)
	res, code, msg := execUpgradeTxIndex(paths, []json.RawMessage{p})
	if code != 0 {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	if _, ok := res.(map[string]interface{}); !ok {
		t.Fatalf("result %#v", res)
	}
}

func TestExecUpgradeTxIndex_NoChainDir(t *testing.T) {
	_, code, msg := execUpgradeTxIndex(nil, nil)
	if code != -1 {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
}

func TestExecUpgradeTxIndexIdempotentSecondPass(t *testing.T) {
	dir := t.TempDir()
	paths := &DataPaths{ChainDataDir: dir}
	res1, code, msg := execUpgradeTxIndex(paths, nil)
	if code != 0 {
		t.Fatalf("first code=%d msg=%s", code, msg)
	}
	res2, code, msg := execUpgradeTxIndex(paths, nil)
	if code != 0 {
		t.Fatalf("second code=%d msg=%s", code, msg)
	}
	m1, ok := res1.(map[string]interface{})
	if !ok {
		t.Fatalf("first %#v", res1)
	}
	m2, ok := res2.(map[string]interface{})
	if !ok {
		t.Fatalf("second %#v", res2)
	}
	if m1["upgraded"] != m2["upgraded"] {
		u1, u2 := m1["upgraded"], m2["upgraded"]
		switch a := u1.(type) {
		case int:
			if b, ok := u2.(int); !ok || a != b {
				t.Fatalf("upgraded drift %v vs %v", u1, u2)
			}
		case float64:
			if b, ok := u2.(float64); !ok || int(a) != int(b) {
				t.Fatalf("upgraded drift %v vs %v", u1, u2)
			}
		default:
			t.Fatalf("upgraded type %T", u1)
		}
	}
}
