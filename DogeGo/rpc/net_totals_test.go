// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestExecGetNetTotalsP2PDisabled(t *testing.T) {
	_, code, msg := execGetNetTotals(nil)
	if code != CodeRPCP2PDisabled || msg != ErrP2PDisabled {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecGetNetTotalsWired(t *testing.T) {
	paths := &DataPaths{
		NetRecv: func() int64 { return 42 },
		NetSent: func() int64 { return 7 },
	}
	res, code, msg := execGetNetTotals(paths)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	if res["totalbytesrecv"].(int64) != 42 {
		t.Fatalf("recv %#v", res["totalbytesrecv"])
	}
	ut, ok := res["uploadtarget"].(map[string]interface{})
	if !ok || ut["serve_historical_blocks"].(bool) != true {
		t.Fatalf("uploadtarget %#v", res["uploadtarget"])
	}
}
