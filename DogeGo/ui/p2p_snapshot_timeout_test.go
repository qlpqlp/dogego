// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"
	"time"
)

func TestP2PSnapshotWithTimeoutReturnsNilOnSlowSnapshot(t *testing.T) {
	s := p2PSnapshotWithTimeout(func() map[string]any {
		time.Sleep(200 * time.Millisecond)
		return map[string]any{"ok": true}
	})
	if s == nil {
		t.Fatal("expected snapshot within 5s timeout")
	}
	s2 := p2PSnapshotWithTimeout(func() map[string]any {
		time.Sleep(50 * time.Millisecond)
		return map[string]any{"ok": true}
	})
	if s2 == nil || s2["ok"] != true {
		t.Fatalf("got %#v", s2)
	}
}
