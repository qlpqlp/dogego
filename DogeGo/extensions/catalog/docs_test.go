// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package catalog

import "testing"

func TestReadMarkdownBuilding(t *testing.T) {
	for _, p := range []string{
		"extensions/catalog/BUILDING.md",
		"extensions/catalog/BUILDING.md",
		"BUILDING.md",
	} {
		b, name, err := ReadMarkdown(p)
		if err != nil {
			t.Fatalf("%q: %v", p, err)
		}
		if len(b) < 100 {
			t.Fatalf("%q: short content", p)
		}
		if name != PathPrefix+"BUILDING.md" {
			t.Fatalf("%q name %q", p, name)
		}
	}
}

func TestNormalizeDocPathLegacy(t *testing.T) {
	got := NormalizeDocPath("extensions/catalog/zkl2/docs/USER_GUIDE.md")
	if got != "zkl2/docs/USER_GUIDE.md" {
		t.Fatalf("got %q", got)
	}
}
