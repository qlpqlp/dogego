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

func TestExecGetAddrmanInfo(t *testing.T) {
	paths := &DataPaths{
		AddrManInfo: func() map[string]interface{} {
			return map[string]interface{}{
				"all": map[string]interface{}{"total": 3, "new": 2, "tried": 1},
			}
		},
	}
	res, code, msg := execGetAddrmanInfo(paths, nil)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("type %T", res)
	}
	all, _ := m["all"].(map[string]interface{})
	if all["total"] != 3 {
		t.Fatalf("total %v", all["total"])
	}
	_, code, _ = execGetAddrmanInfo(paths, []json.RawMessage{json.RawMessage(`1`)})
	if code != -8 {
		t.Fatalf("want -8 for extra arg, got %d", code)
	}
}

func TestExecGetAddrmanInfoP2PDisabled(t *testing.T) {
	_, code, _ := execGetAddrmanInfo(nil, nil)
	if code != CodeRPCP2PDisabled {
		t.Fatalf("code %d", code)
	}
}
