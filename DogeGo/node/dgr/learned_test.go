// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dgr

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLearnedRelayStoreNoteAndList(t *testing.T) {
	dir := t.TempDir()
	s := OpenLearnedRelayStore(dir)
	if !s.Note("203.0.113.9", 24433) {
		t.Fatal("expected new note")
	}
	if s.Note("203.0.113.9:24433", 24433) {
		t.Fatal("duplicate should be false")
	}
	list := s.List()
	if len(list) != 1 || list[0] != "203.0.113.9:24433" {
		t.Fatalf("list %#v", list)
	}
	s2 := OpenLearnedRelayStore(dir)
	if len(s2.List()) != 1 {
		t.Fatalf("reload %#v", s2.List())
	}
	if _, err := filepath.Abs(filepath.Join(dir, learnedRelaysFileName)); err != nil {
		t.Fatal(err)
	}
}

func TestMergeRelaySeedLists(t *testing.T) {
	got := MergeRelaySeedLists([]string{"a.example:24433"}, []string{"b.example:24433", "a.example:24433"}, 32)
	if len(got) != 2 {
		t.Fatalf("%v", got)
	}
}

func TestShuffleSecurePermutes(t *testing.T) {
	in := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	cp := append([]string(nil), in...)
	changed := false
	for i := 0; i < 20; i++ {
		ShuffleSecure(cp)
		for j := range in {
			if cp[j] != in[j] {
				changed = true
				break
			}
		}
		if changed {
			break
		}
		cp = append([]string(nil), in...)
	}
	if !changed {
		t.Fatal("expected shuffle to change order")
	}
}

func TestDiscoverTargetsIncludesLearned(t *testing.T) {
	out := DiscoverTargets(context.Background(), "", []string{"seed:24433"}, []string{"learned:24433"}, 24433, nil)
	if len(out) != 2 {
		t.Fatalf("%v", out)
	}
}
