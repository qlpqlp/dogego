// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTipWaiterWaitNewBlock(t *testing.T) {
	tw := NewTipWaiter()
	tw.Notify(1, "aa")
	paths := &DataPaths{TipWaiter: tw}
	go func() {
		time.Sleep(30 * time.Millisecond)
		tw.Notify(2, "bb")
	}()
	res, code, msg := execWaitForNewBlock(paths, nil, nil, nil)
	if code != 0 {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("result %#v", res)
	}
	h, _ := m["height"].(int64)
	if h == 0 {
		h = int64(m["height"].(float64))
	}
	if h != 2 {
		t.Fatalf("height %#v", m["height"])
	}
}

func TestTipWaiterWaitBlockHeightImmediate(t *testing.T) {
	tw := NewTipWaiter()
	tw.Notify(10, "ff")
	paths := &DataPaths{TipWaiter: tw}
	res, code, msg := execWaitForBlockHeight(paths, nil, nil, mustRawJSON(t, `5`))
	if code != 0 {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	m := res.(map[string]interface{})
	h, _ := m["height"].(int64)
	if h == 0 {
		h = int64(m["height"].(float64))
	}
	if h != 10 {
		t.Fatalf("height %#v", m)
	}
}

func mustRawJSON(t *testing.T, s string) []json.RawMessage {
	t.Helper()
	return []json.RawMessage{json.RawMessage(s)}
}
