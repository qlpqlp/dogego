// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package node

import (
	"testing"
	"time"
)

func TestUncleanShutdownMarkerRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if HasUncleanShutdown(dir) {
		t.Fatal("expected clean")
	}
	MarkUncleanShutdown(dir)
	if !HasUncleanShutdown(dir) {
		t.Fatal("expected unclean")
	}
	ClearUncleanShutdown(dir)
	if HasUncleanShutdown(dir) {
		t.Fatal("expected cleared")
	}
}

func TestRunWithTimeout(t *testing.T) {
	if !RunWithTimeout(time.Second, func() {}) {
		t.Fatal("expected finish")
	}
	if RunWithTimeout(20*time.Millisecond, func() { time.Sleep(200 * time.Millisecond) }) {
		t.Fatal("expected timeout")
	}
}
