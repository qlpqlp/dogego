// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"strings"
	"testing"
)

func TestSyncStatusLineNotUpToDateWhenBodiesLag(t *testing.T) {
	line := SyncStatusLine("full", "block_chain_connected", 4000, 0, 0.01, 4000, "", 0)
	if line == "Up to date" || line == "" {
		t.Fatalf("got %q", line)
	}
	if !strings.Contains(line, "Synchronizing blocks") || !strings.Contains(line, "behind") {
		t.Fatalf("got %q", line)
	}
}

func TestDogeGoSyncPhaseForwardWhenContiguousLags(t *testing.T) {
	if p := DogeGoSyncPhase("full", 4000, 4000, 9, false); p != "forward_block_ibd" {
		t.Fatalf("phase %q want forward_block_ibd", p)
	}
}

func TestBlocksBehindHeadersUsesContiguous(t *testing.T) {
	if b := BlocksBehindHeaders(100, 100, 9); b != 91 {
		t.Fatalf("behind %d want 91", b)
	}
}

func TestHeadersSyncProgress(t *testing.T) {
	if p := HeadersSyncProgress(369886, 1); p <= 0 || p >= 1 {
		t.Fatalf("mid-IBD progress %v want (0,1)", p)
	}
	if HeadersSyncProgress(100, 100) != 1 {
		t.Fatal("caught up should be 1")
	}
}

func TestSyncHealthNotHealthyWhenBodiesFarBehind(t *testing.T) {
	health, ok := SyncHealthAssessment("block_chain_connected", 4000, 4000, 3000, 0, 0, "", false)
	if health == "healthy" && ok {
		t.Fatalf("health %q ok=%v want not healthy when behind>>32", health, ok)
	}
}

func TestSyncHealthForwardIBDActiveWhenHeadersPaused(t *testing.T) {
	health, ok := SyncHealthAssessment("forward_block_ibd", 534_000, 616, 533_000, 0.5, 0, "stale tip", true)
	if health != "forward_ibd_active" || !ok {
		t.Fatalf("health %q ok=%v want forward_ibd_active during deep body IBD", health, ok)
	}
}
