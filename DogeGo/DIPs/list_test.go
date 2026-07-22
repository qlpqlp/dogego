// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dips

import (
	"bytes"
	"testing"
)

func TestListHasCoreDIPs(t *testing.T) {
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 10 {
		t.Fatalf("expected many DIPs, got %d", len(list))
	}
	seen := map[int]bool{}
	for _, e := range list {
		seen[e.Number] = true
		if e.ID == "" || e.Title == "" || e.Path == "" {
			t.Fatalf("incomplete entry %#v", e)
		}
	}
	for _, n := range []int{21, 44, 125, 158, 3869} {
		if !seen[n] {
			t.Fatalf("missing DIP-%04d", n)
		}
	}
}

func TestReadMarkdown(t *testing.T) {
	b, name, err := ReadMarkdown("DIPs/dip-0021.md")
	if err != nil {
		t.Fatal(err)
	}
	if name != "DIPs/dip-0021.md" {
		t.Fatalf("name %q", name)
	}
	if !bytes.Contains(b, []byte("DIP-0021")) {
		t.Fatalf("unexpected body")
	}
}
