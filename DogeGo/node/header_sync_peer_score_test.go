// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestHeaderSyncFailureHard(t *testing.T) {
	if !headerSyncFailureHard(syncTestErr("header sync stall: no headers for 30s at tip 1 (peer height 2)")) {
		t.Fatal("stall should be hard")
	}
	if headerSyncFailureHard(syncTestErr("timeout waiting for headers")) != true {
		t.Fatal("timeout should be hard")
	}
	if headerSyncFailureHard(syncTestErr("headers: rewound journal to height 1 (retry getheaders)")) {
		t.Fatal("local rewind should not hard-fail peer")
	}
}

func TestNoteHeaderSyncPeerFailure(t *testing.T) {
	scorer := NewBlockPeerScorer()
	noteHeaderSyncPeerFailure(scorer, nil, "93.184.216.1:22556", syncTestErr("header sync stall: no headers"))
	stats, ok := scorer.Stats("93.184.216.1:22556")
	if !ok || stats.Failures == 0 {
		t.Fatalf("stats %+v ok=%v", stats, ok)
	}
}

type syncTestErr string

func (e syncTestErr) Error() string { return string(e) }
