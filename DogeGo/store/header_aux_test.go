// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"path/filepath"
	"testing"
)

func TestHeaderAuxJournalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "headers_aux.bin")
	a, err := OpenHeaderAuxJournal(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	blob := []byte{0x01, 0x02, 0x03}
	if err := a.AppendEntries([][]byte{nil, blob, nil}); err != nil {
		t.Fatal(err)
	}
	if a.RecordCount() != 3 {
		t.Fatalf("count %d", a.RecordCount())
	}
	got, err := a.ReadAt(1)
	if err != nil || string(got) != string(blob) {
		t.Fatalf("read: %q err=%v", got, err)
	}
	empty, err := a.ReadAt(0)
	if err != nil || empty != nil {
		t.Fatalf("height 0 aux: %v err=%v", empty, err)
	}
	if err := a.TruncateToHeight(0); err != nil {
		t.Fatal(err)
	}
	if a.RecordCount() != 1 {
		t.Fatalf("after truncate %d", a.RecordCount())
	}
}

func TestHeaderAuxJournalReopenAfterManyAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "headers_aux.bin")
	a, err := OpenHeaderAuxJournal(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	const n = 2000
	blobs := make([][]byte, n)
	for i := range blobs {
		if i%7 == 1 {
			blobs[i] = []byte{byte(i), byte(i >> 8), 0xab}
		}
	}
	if err := a.AppendEntries(blobs); err != nil {
		t.Fatal(err)
	}
	a2, err := OpenHeaderAuxJournal(path, int64(n))
	if err != nil {
		t.Fatal(err)
	}
	if a2.RecordCount() != int64(n) {
		t.Fatalf("reopen count %d want %d", a2.RecordCount(), n)
	}
	got, err := a2.ReadAt(43)
	if err != nil || len(got) != 3 || got[0] != 43 {
		t.Fatalf("read 43: %v err=%v", got, err)
	}
}

func TestHeaderAuxTruncateWhenShorterThanHeaders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "headers_aux.bin")
	a, err := OpenHeaderAuxJournal(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	if a.RecordCount() != 100 {
		t.Fatalf("count %d", a.RecordCount())
	}
	// Main journal rewound to height 500 while aux only has 100 records - must not error.
	if err := a.TruncateToHeight(500); err != nil {
		t.Fatal(err)
	}
	if a.RecordCount() != 100 {
		t.Fatalf("after truncate above tip: count %d", a.RecordCount())
	}
	if err := a.TruncateToHeight(50); err != nil {
		t.Fatal(err)
	}
	if a.RecordCount() != 51 {
		t.Fatalf("after truncate down: count %d", a.RecordCount())
	}
}

func TestHeaderAuxEnsureRecordCountPadsAndTruncates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "headers_aux.bin")
	a, err := OpenHeaderAuxJournal(path, 5)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.EnsureRecordCount(10); err != nil {
		t.Fatal(err)
	}
	if a.RecordCount() != 10 {
		t.Fatalf("pad count %d", a.RecordCount())
	}
	if err := a.EnsureRecordCount(3); err != nil {
		t.Fatal(err)
	}
	if a.RecordCount() != 3 {
		t.Fatalf("truncate count %d", a.RecordCount())
	}
}

func TestHeaderAuxJournalPadToHeaders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "headers_aux.bin")
	a, err := OpenHeaderAuxJournal(path, 3)
	if err != nil {
		t.Fatal(err)
	}
	if a.RecordCount() != 3 {
		t.Fatalf("padded count %d", a.RecordCount())
	}
}
