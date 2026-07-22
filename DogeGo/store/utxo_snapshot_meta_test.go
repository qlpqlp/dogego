// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"os"
	"testing"
)

func TestReadUtxoSnapshotDiskTip(t *testing.T) {
	dir := t.TempDir()
	path := UtxoSnapshotPath(dir)
	if tip, err := ReadUtxoSnapshotDiskTip(path); err != nil || tip != -1 {
		t.Fatalf("missing file tip=%d err=%v want -1 nil", tip, err)
	}
	u := NewUtxoCache()
	u.SetTipHeightForTest(10403)
	if err := u.SaveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	tip, err := ReadUtxoSnapshotDiskTip(path)
	if err != nil || tip != 10403 {
		t.Fatalf("tip=%d err=%v want 10403", tip, err)
	}
	_, mod, err := ReadUtxoSnapshotDiskMeta(path)
	if err != nil || mod <= 0 {
		t.Fatalf("mod=%d err=%v", mod, err)
	}
	st, _ := os.Stat(path)
	if mod != st.ModTime().Unix() {
		t.Fatalf("mod unix mismatch")
	}
}
