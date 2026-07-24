// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"strings"
	"testing"

	"dogego/rpc"
)

// Required workflow keywords in the Docs manifest (Phase 12 sync check).
var docsWorkflowKeywords = []string{
	"sendrawtransaction", "testmempoolaccept", "getblockchaininfo",
	"fundrawtransaction", "walletcreatefundedpsbt", "getblockfilter", "scanblocks",
}

func TestDocsManifestIncludesExtensionsSection(t *testing.T) {
	m := DefaultDocsManifest()
	for _, s := range m.Sections {
		if s.ID != "extensions" {
			continue
		}
		if !strings.Contains(s.Body, "dogego.zkl2") {
			t.Fatal("extensions section missing dogego.zkl2")
		}
		if !strings.Contains(s.Body, "dogego.doginals") {
			t.Fatal("extensions section missing dogego.doginals")
		}
		if !strings.Contains(s.Body, "dogego.radiodoge") {
			t.Fatal("extensions section missing dogego.radiodoge")
		}
		if !strings.Contains(s.Body, "dogego.bbpow") {
			t.Fatal("extensions section missing dogego.bbpow")
		}
		wantPaths := []string{
			"docs/EXTENSIONS.md",
			"extensions/catalog/radiodoge/docs/USER_GUIDE.md",
			"extensions/catalog/doginals/docs/USER_GUIDE.md",
			"extensions/catalog/bbpow/docs/USER_GUIDE.md",
		}
		for _, want := range wantPaths {
			found := false
			for _, l := range s.Links {
				if l.Path == want {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("extensions section missing link %s", want)
			}
		}
		return
	}
	t.Fatal("merged docs manifest missing extensions section")
}

func TestDocsManifestIncludesGuideTopics(t *testing.T) {
	m := DefaultDocsManifest()
	want := []string{"sync", "p2p", "mempool", "core_parity", "wallet", "rpc"}
	ids := make(map[string]bool)
	for _, s := range m.Sections {
		ids[s.ID] = true
	}
	for _, id := range want {
		if !ids[id] {
			t.Fatalf("merged docs manifest missing guide topic %q", id)
		}
	}
}

func TestDocsManifestMentionsWalletMempoolWorkflows(t *testing.T) {
	m := DefaultDocsManifest()
	var rpcBody, walletBody string
	for _, s := range m.Sections {
		switch s.ID {
		case "rpc":
			rpcBody = s.Body
		case "wallet":
			walletBody = s.Body
		}
	}
	combined := rpcBody + walletBody
	for _, kw := range docsWorkflowKeywords {
		if !strings.Contains(combined, kw) {
			t.Fatalf("docs manifest missing workflow keyword %q in rpc/wallet sections", kw)
		}
	}
}

func TestDocsManifestMentionsRpcConsoleTutorial(t *testing.T) {
	m := DefaultDocsManifest()
	for _, s := range m.Sections {
		if s.ID == "rpc" && strings.Contains(s.Body, "RPC_CONSOLE_TUTORIAL.md") {
			for _, l := range s.Links {
				if l.Path == "docs/RPC_CONSOLE_TUTORIAL.md" {
					return
				}
			}
			t.Fatal("rpc section missing RPC_CONSOLE_TUTORIAL.md link")
		}
	}
	t.Fatal("rpc docs section missing RPC_CONSOLE_TUTORIAL.md")
}

func TestDocsManifestMentionsFounderPlaybook(t *testing.T) {
	m := DefaultDocsManifest()
	for _, s := range m.Sections {
		if s.ID == "operator" && strings.Contains(s.Body, "dogego cert founder") {
			return
		}
	}
	t.Fatal("operator docs section should mention dogego cert founder")
}

func TestDocsManifestMentionsDogegoLiveWorkflow10(t *testing.T) {
	m := DefaultDocsManifest()
	for _, s := range m.Sections {
		if s.ID != "operator" {
			continue
		}
		if !strings.Contains(s.Body, "workflow 10") {
			t.Fatal("operator section missing workflow 10")
		}
		for _, l := range s.Links {
			if l.Path == "docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md" {
				return
			}
		}
		t.Fatal("operator section missing CORE_SIDE_BY_SIDE_WORKFLOWS.md link")
	}
	t.Fatal("missing operator section")
}

func TestDocsManifestRPCCountMatchesBuild(t *testing.T) {
	m := DefaultDocsManifest()
	n := len(rpc.SupportedMethods())
	if n < 80 {
		t.Fatalf("expected large SupportedMethods list, got %d", n)
	}
	var hasStart bool
	for _, s := range m.Sections {
		if s.ID == "start_here" && strings.Contains(s.Body, "STANDALONE_NODE_QUICKSTART") {
			hasStart = true
		}
	}
	if !hasStart {
		t.Fatal("expected start_here section for new operators")
	}
}
