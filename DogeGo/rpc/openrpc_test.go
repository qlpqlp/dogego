// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestBuildOpenRPCDocumentCoversSupportedMethods(t *testing.T) {
	doc := BuildOpenRPCDocument()
	if doc.OpenRPC != "1.3.2" {
		t.Fatalf("openrpc version %q", doc.OpenRPC)
	}
	if len(doc.Methods) != len(SupportedMethods()) {
		t.Fatalf("methods %d want %d", len(doc.Methods), len(SupportedMethods()))
	}
	seen := make(map[string]struct{}, len(doc.Methods))
	for _, m := range doc.Methods {
		if m.Name == "" {
			t.Fatal("empty method name")
		}
		if m.Summary == "" && m.Description == "" {
			t.Fatalf("method %q missing help", m.Name)
		}
		seen[m.Name] = struct{}{}
	}
	for _, m := range SupportedMethods() {
		if _, ok := seen[m]; !ok {
			t.Fatalf("openrpc missing %q", m)
		}
	}
}

func TestBuildRPCCookbookCoversSupportedMethods(t *testing.T) {
	book := BuildRPCCookbook()
	if len(book) != len(SupportedMethods()) {
		t.Fatalf("cookbook %d want %d", len(book), len(SupportedMethods()))
	}
	for _, e := range book {
		if e.Method == "" || e.Curl == "" || e.CLI == "" {
			t.Fatalf("incomplete entry for %q", e.Method)
		}
		if e.Params == nil {
			t.Fatalf("entry %q missing params slice", e.Method)
		}
		if !stringsContains(e.Curl, e.Method) {
			t.Fatalf("curl for %q missing method name", e.Method)
		}
	}
}

func TestCookbookExampleParamsKnownMethods(t *testing.T) {
	addnode := CookbookExampleParams("addnode")
	if len(addnode) != 2 {
		t.Fatalf("addnode params len %d", len(addnode))
	}
	if addnode[0] != "HOST:44556" || addnode[1] != "add" {
		t.Fatalf("addnode params %#v", addnode)
	}
	verify := CookbookExampleParams("verifychain")
	if len(verify) != 1 || verify[0] != 3 {
		t.Fatalf("verifychain params %#v", verify)
	}
	if len(CookbookExampleParams("getblockchaininfo")) != 0 {
		t.Fatal("getblockchaininfo should have empty params")
	}
}

func stringsContains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
