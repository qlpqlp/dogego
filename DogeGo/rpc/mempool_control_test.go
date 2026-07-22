// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"

	"dogego/mempool"
)

func TestExecSetMempoolPaused(t *testing.T) {
	p := mempool.New(8)
	ok, code, msg := execSetMempoolPaused(p, []json.RawMessage{json.RawMessage(`true`)})
	if code != 0 || !ok || msg != "" || !p.Paused() {
		t.Fatalf("pause code=%d ok=%v msg=%q paused=%v", code, ok, msg, p.Paused())
	}
	ok, code, msg = execSetMempoolPaused(p, []json.RawMessage{json.RawMessage(`false`)})
	if code != 0 || !ok || p.Paused() {
		t.Fatalf("resume code=%d ok=%v paused=%v", code, ok, p.Paused())
	}
}

func TestExecSetMempoolPaused_noPool(t *testing.T) {
	_, code, _ := execSetMempoolPaused(nil, []json.RawMessage{json.RawMessage(`true`)})
	if code != -18 {
		t.Fatalf("code=%d", code)
	}
}
