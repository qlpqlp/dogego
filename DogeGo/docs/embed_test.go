// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package docs

import "testing"

func TestReadMarkdownOperator(t *testing.T) {
	b, name, err := ReadMarkdown("docs/OPERATOR.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 100 {
		t.Fatal("expected OPERATOR.md content")
	}
	if name != "docs/OPERATOR.md" {
		t.Fatalf("name %q", name)
	}
}

func TestReadMarkdownRejectsTraversal(t *testing.T) {
	_, _, err := ReadMarkdown("../secrets.md")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMarkdownNamesNonEmpty(t *testing.T) {
	n, err := MarkdownNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(n) == 0 {
		t.Fatal("expected embedded doc list")
	}
}
