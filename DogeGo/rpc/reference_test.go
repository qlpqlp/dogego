// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"strings"
	"testing"
)

func TestBuildRPCReferenceHTML(t *testing.T) {
	html := BuildRPCReferenceHTML()
	if !strings.Contains(html, "getblockchaininfo") {
		t.Fatal("missing method in reference HTML")
	}
	if !strings.Contains(html, "<table>") {
		t.Fatal("expected table")
	}
}

func TestBuildIntegrationGuidesLanguages(t *testing.T) {
	guides := BuildIntegrationGuides()
	if len(guides) < 5 {
		t.Fatalf("guides %d want >= 5", len(guides))
	}
	seen := make(map[string]struct{})
	for _, g := range guides {
		if g.Language == "" || g.Example == "" {
			t.Fatalf("incomplete guide %+v", g)
		}
		seen[g.Language] = struct{}{}
	}
	for _, lang := range []string{"curl", "python", "go", "node", "rust"} {
		if _, ok := seen[lang]; !ok {
			t.Fatalf("missing language %q", lang)
		}
	}
}
