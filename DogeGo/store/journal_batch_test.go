// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
)

func TestHeaderJournalBatchAppend(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := OpenHeaderJournal(filepath.Join(dir, "h.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	const n = 2000
	batch := make([][]byte, n)
	for i := 0; i < n; i++ {
		h := append([]byte(nil), g80[:]...)
		h[76] = byte(i)
		batch[i] = h
	}
	if err := j.AppendHeaders(batch); err != nil {
		t.Fatal(err)
	}
	cnt, err := j.Count()
	if err != nil || cnt != int64(n+1) {
		t.Fatalf("count %d err %v", cnt, err)
	}
}
