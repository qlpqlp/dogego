// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"
	"time"
)

func TestShouldDeferBackgroundHeaderSync(t *testing.T) {
	dedicatedHeaderSyncRunning.Store(0)
	syncActivity.mu.Lock()
	syncActivity.lastProgressAt = time.Time{}
	syncActivity.lastKind = ""
	syncActivity.mu.Unlock()

	if ShouldDeferBackgroundHeaderSync() {
		t.Fatal("expected false when dedicated not running")
	}

	dedicatedHeaderSyncRunning.Store(1)
	defer dedicatedHeaderSyncRunning.Store(0)

	if ShouldDeferBackgroundHeaderSync() {
		t.Fatal("expected false when dedicated running but no recent header progress")
	}

	NoteHeadersAppended(1, 100)
	if !ShouldDeferBackgroundHeaderSync() {
		t.Fatal("expected true when dedicated running and headers just advanced")
	}

	syncActivity.mu.Lock()
	syncActivity.lastProgressAt = time.Now().Add(-5 * time.Minute)
	syncActivity.mu.Unlock()
	if ShouldDeferBackgroundHeaderSync() {
		t.Fatal("expected false when header progress is stale")
	}
}
