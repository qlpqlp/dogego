// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestHeadersSyncMaxRoundsWhenBodiesBehind(t *testing.T) {
	if headersSyncMaxRounds(1_000_000, 1_000_000, true) >= 4096 {
		t.Fatal("expected bounded rounds when peer at local tip but bodies lag")
	}
	if headersSyncMaxRounds(1_000_000, 1_100_000, true) != 4096 {
		t.Fatal("peer ahead on headers: full header sync")
	}
	if headersSyncMaxRounds(100, 100, false) != 4096 {
		t.Fatal("bodies caught up: normal header rounds")
	}
}

func TestHasLocalHeaderChain(t *testing.T) {
	if HasLocalHeaderChain(nil) {
		t.Fatal("nil journal")
	}
}

func TestShouldDeferHeaderSyncWhileBodiesLag(t *testing.T) {
	if !ShouldDeferHeaderSyncWhileBodiesLag(1_598_000, 1_598_000, true) {
		t.Fatal("peer at tip with bodies behind")
	}
	if ShouldDeferHeaderSyncWhileBodiesLag(1_598_000, 1_600_000, true) {
		t.Fatal("peer far ahead should still fetch headers")
	}
	if ShouldDeferHeaderSyncWhileBodiesLag(100, 100, false) {
		t.Fatal("caught up bodies")
	}
}

func TestPrepareAtStartupArmsSync(t *testing.T) {
	var s progressiveRawState
	// Without a real journal/store, PrepareAtStartup is a no-op; test idle flag via LowestMissing path in integration tests.
	s.idleFull = true
	if s.useShortReadDeadline() {
		t.Fatal("idle should not fetch")
	}
	s.idleFull = false
	if !s.useShortReadDeadline() {
		t.Fatal("active sync should fetch")
	}
}
