// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecoverHeaderAuxJournal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "headers_aux.bin")
	if err := os.WriteFile(path, []byte{0xff, 0xff, 0xff, 0xff}, 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := RecoverHeaderAuxJournal(path, 5)
	if err != nil {
		t.Fatal(err)
	}
	if a.RecordCount() != 5 {
		t.Fatalf("count %d", a.RecordCount())
	}
	if _, err := os.Stat(path + ".corrupt"); err != nil {
		t.Fatal("expected .corrupt backup")
	}
	if _, err := OpenHeaderAuxJournal(path, 5); err != nil {
		t.Fatal(err)
	}
}

func TestHeaderAuxJournalRepairsTornTail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "headers_aux.bin")
	a, err := OpenHeaderAuxJournal(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	blob := []byte{0xde, 0xad, 0xbe, 0xef}
	if err := a.AppendEntries([][]byte{nil, blob, nil, nil}); err != nil {
		t.Fatal(err)
	}
	if a.RecordCount() != 4 {
		t.Fatalf("count %d want 4", a.RecordCount())
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte{0xff, 0xff, 0xff, 0xff}); err != nil { // torn partial record
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	a2, err := OpenHeaderAuxJournal(path, 4)
	if err != nil {
		t.Fatal(err)
	}
	if a2.RecordCount() != 4 {
		t.Fatalf("after tail repair count=%d want 4", a2.RecordCount())
	}
	got, err := a2.ReadAt(1)
	if err != nil || string(got) != string(blob) {
		t.Fatalf("record 1: %q err=%v", got, err)
	}
}
