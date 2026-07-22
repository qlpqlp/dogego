// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLearnedAddrsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "learned_addrs.json")
	in := []string{"1.2.3.4:22556", "seed.example.com:22556", "1.2.3.4:22556"}
	if err := SaveLearnedAddrs(path, in); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLearnedAddrs(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "1.2.3.4:22556" || got[1] != "seed.example.com:22556" {
		t.Fatalf("got %v", got)
	}
}

func TestLoadLearnedAddrsMissing(t *testing.T) {
	got, err := LoadLearnedAddrs(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v err %v", got, err)
	}
}

func TestAddrBookV2RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "learned_addrs.json")
	b := NewAddrBook()
	b.AddSeen("1.2.3.4:22556")
	b.NoteTry("1.2.3.4:22556")
	b.NoteSuccess("1.2.3.4:22556")
	if err := SaveAddrBook(path, b); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadAddrBook(path)
	if err != nil {
		t.Fatal(err)
	}
	snap := loaded.Snapshot()
	if len(snap) != 1 || snap[0] != "1.2.3.4:22556" {
		t.Fatalf("snap %v", snap)
	}
	loaded.mu.Lock()
	rec := loaded.by["1.2.3.4:22556"]
	loaded.mu.Unlock()
	if rec == nil || rec.Successes != 1 {
		t.Fatalf("rec %+v", rec)
	}
	if !loaded.HasAddrmanKey() {
		t.Fatal("expected nKey after v3 save")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"version": 3`) || !strings.Contains(string(raw), `"n_key"`) {
		t.Fatalf("expected v3 n_key file: %s", string(raw))
	}
}
