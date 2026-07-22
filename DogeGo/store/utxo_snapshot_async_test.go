// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"testing"
	"time"
)

func TestSaveSnapshotAsync(t *testing.T) {
	u := NewUtxoCache()
	u.SetTipHeightForTest(42)
	dir := t.TempDir()
	path := UtxoSnapshotPath(dir)
	started, err := u.SaveSnapshotAsync(path)
	if err != nil || !started {
		t.Fatalf("started=%v err=%v", started, err)
	}
	started2, _ := u.SaveSnapshotAsync(path)
	if started2 {
		t.Fatal("expected second async save to be deduped")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !UtxoSnapshotSaveInFlight() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if UtxoSnapshotSaveInFlight() {
		t.Fatal("snapshot still in flight")
	}
	tip, err := ReadUtxoSnapshotDiskTip(path)
	if err != nil || tip != 42 {
		t.Fatalf("disk tip=%d err=%v", tip, err)
	}
}
