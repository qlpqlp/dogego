// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package docs

import (
	"strings"
	"testing"
)

func TestResolveMarkdownLinkRelative(t *testing.T) {
	p, a, ext, err := ResolveMarkdownLink("docs/CORE_PARITY_GAPS.md", "INTENTIONAL_DIFFERENCES.md")
	if err != nil || ext || a != "" {
		t.Fatalf("got %q %q ext=%v err=%v", p, a, ext, err)
	}
	if p != "docs/INTENTIONAL_DIFFERENCES.md" {
		t.Fatalf("path %q", p)
	}
}

func TestResolveMarkdownLinkParentROADMAP(t *testing.T) {
	p, _, ext, err := ResolveMarkdownLink("docs/CORE_PARITY_GAPS.md", "../ROADMAP.md")
	if err != nil || ext {
		t.Fatalf("err=%v ext=%v", err, ext)
	}
	if p != "ROADMAP.md" {
		t.Fatalf("path %q", p)
	}
}

func TestResolveMarkdownLinkAnchorOnly(t *testing.T) {
	_, a, ext, err := ResolveMarkdownLink("docs/OPERATOR.md", "#security")
	if err != nil || ext || a != "#security" {
		t.Fatalf("anchor %q ext=%v err=%v", a, ext, err)
	}
}

func TestResolveMarkdownLinkExternal(t *testing.T) {
	_, _, ext, err := ResolveMarkdownLink("docs/OPERATOR.md", "https://github.com/dogecoin/dogecoin")
	if err != nil || !ext {
		t.Fatalf("ext=%v err=%v", ext, err)
	}
}

func TestResolveMarkdownLinkRejectsTraversal(t *testing.T) {
	_, _, _, err := ResolveMarkdownLink("docs/OPERATOR.md", "../../../secrets.md")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveMarkdownLinkCatalogRelative(t *testing.T) {
	p, a, ext, err := ResolveMarkdownLink("extensions/catalog/doginals/docs/USER_GUIDE.md", "PROTOCOL.md")
	if err != nil || ext || a != "" {
		t.Fatalf("got %q %q ext=%v err=%v", p, a, ext, err)
	}
	if p != "extensions/catalog/doginals/docs/PROTOCOL.md" {
		t.Fatalf("path %q", p)
	}
}

func TestReadMarkdownBitcoinWhitepaper(t *testing.T) {
	b, name, err := ReadMarkdown("docs/BITCOIN_WHITEPAPER.md")
	if err != nil {
		t.Fatalf("ReadMarkdown: %v", err)
	}
	if name != "docs/BITCOIN_WHITEPAPER.md" {
		t.Fatalf("name %q", name)
	}
	if len(b) < 1000 || !strings.Contains(string(b), "Satoshi Nakamoto") {
		t.Fatalf("unexpected whitepaper content len=%d", len(b))
	}
}

func TestReadMarkdownROADMAPFromModuleRoot(t *testing.T) {
	b, name, err := ReadMarkdown("ROADMAP.md")
	if err != nil {
		t.Skipf("ROADMAP.md not available in this environment: %v", err)
	}
	if len(b) < 500 || name != "ROADMAP.md" {
		t.Fatalf("name=%q len=%d", name, len(b))
	}
}
