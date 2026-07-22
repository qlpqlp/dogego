// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestExecGetRPCInfoCoreFields(t *testing.T) {
	before := RPCAuthFailures()
	RecordRPCAuthFailure()
	RecordRPCAuthFailure()
	out := execGetRPCInfo(&DataPaths{RPCTLSEnabled: true, Uptime: func() int64 { return 42 }})
	if RPCAuthFailures() != before+2 {
		t.Fatalf("auth failures %d want %d", RPCAuthFailures(), before+2)
	}
	method, ok := out["method"].(map[string]interface{})
	if !ok || len(method) < 10 {
		t.Fatalf("method map %#v", out["method"])
	}
	if _, ok := method["getblockchaininfo"]; !ok {
		t.Fatal("missing getblockchaininfo in method map")
	}
	if out["dogego_rpc_tls"].(bool) != true {
		t.Fatalf("tls %#v", out["dogego_rpc_tls"])
	}
	if out["dogego_uptime_seconds"].(int64) != 42 {
		t.Fatalf("uptime %#v", out["dogego_uptime_seconds"])
	}
}
