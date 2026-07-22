// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"os"
	"path/filepath"
	"testing"

	"dogego/rpc"
)

func TestMisbehaviorPersistRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "misbehavior_scores.json")
	m1 := NewMisbehaviorTracker(rpc.NewMemoryBanManager())
	m1.Note("203.0.113.9:22556", 15, "test")
	if err := SaveMisbehaviorScores(m1, path); err != nil {
		t.Fatal(err)
	}
	m2 := NewMisbehaviorTracker(rpc.NewMemoryBanManager())
	LoadMisbehaviorScores(m2, path)
	if got := m2.Score("203.0.113.9:22556"); got != 15 {
		t.Fatalf("score %d", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
