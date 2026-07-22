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

func TestBuildChainStatsNilJournal(t *testing.T) {
	m := BuildChainStats(nil, nil, 0x71, time.Now(), -1, -1, false)
	if m["error"] != "no journal" {
		t.Fatalf("got %#v", m["error"])
	}
}
