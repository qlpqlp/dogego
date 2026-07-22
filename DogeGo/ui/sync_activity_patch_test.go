// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "testing"

func TestPatchSyncActivityRespectsBodyIBDPause(t *testing.T) {
	act := map[string]any{"headline": "old"}
	patchSyncActivityForHeaderTip(act, 534_000, 6_264_746, 744, 744, 533_256, 2.4, 745, 1, true)
	if got := act["headline"]; got != "Downloading block bodies from height 745" {
		t.Fatalf("headline %q want body download during pause", got)
	}
}

func TestPatchSyncActivityHeaderCatchUpWhenNotPaused(t *testing.T) {
	act := map[string]any{}
	patchSyncActivityForHeaderTip(act, 10_000, 6_264_746, 9_000, 9_000, 1000, 0, 9001, 0, false)
	if got := act["headline"]; got != "Catching up headers (10000 / ~6264746)" {
		t.Fatalf("headline %q want header catch-up", got)
	}
}
