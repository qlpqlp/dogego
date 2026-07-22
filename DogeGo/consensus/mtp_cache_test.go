// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/store"
)

func TestMedianTimePastAtCache(t *testing.T) {
	ClearMedianTimePastCache()
	dir := t.TempDir()
	blockRaw, _ := store.TestMinimalBlock()
	j, err := store.OpenHeaderJournal(dir+"/headers.bin", blockRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		if err := j.AppendHeaders([][]byte{blockRaw[:80]}); err != nil {
			t.Fatal(err)
		}
	}
	m1, err := MedianTimePastAt(j, 10)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := MedianTimePastAt(j, 10)
	if err != nil || m1 != m2 {
		t.Fatalf("cache miss m1=%d m2=%d err=%v", m1, m2, err)
	}
	ClearMedianTimePastCache()
	m3, err := MedianTimePastAt(j, 10)
	if err != nil || m3 != m1 {
		t.Fatalf("after clear m3=%d want %d", m3, m1)
	}
}
