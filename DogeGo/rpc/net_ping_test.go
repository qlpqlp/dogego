// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestExecPingWired(t *testing.T) {
	var called bool
	paths := &DataPaths{PingPeers: func() { called = true }}
	_, code, msg := execPing(paths)
	if code != 0 {
		t.Fatalf("%s", msg)
	}
	if !called {
		t.Fatal("PingPeers not called")
	}
}

func TestExecPingDisabled(t *testing.T) {
	_, code, _ := execPing(nil)
	if code == 0 {
		t.Fatal("expected error")
	}
}
