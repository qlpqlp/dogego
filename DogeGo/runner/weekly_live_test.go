// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindModuleRoot(t *testing.T) {
	root, err := FindModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
}

func TestRunWeeklyLiveWrongNetwork(t *testing.T) {
	r := RunWeeklyLive(WeeklyLiveOptions{Network: "mainnet"})
	if r.OK || len(r.Issues) == 0 {
		t.Fatalf("%+v", r)
	}
	if r.Doc != DogegoLiveWorkflow10Doc {
		t.Fatalf("doc %q", r.Doc)
	}
}

func TestRunLiveSoakWrongNetwork(t *testing.T) {
	r := RunLiveSoak(LiveSoakOptions{Network: "mainnet"})
	if r.OK || len(r.Issues) == 0 {
		t.Fatalf("%+v", r)
	}
	if r.Doc != DogegoLiveWorkflow10Doc {
		t.Fatalf("doc %q", r.Doc)
	}
}

func TestRunWeeklyLiveSkipScriptsNeedsLiveNode(t *testing.T) {
	if !hasGo() {
		t.Skip("go not in PATH")
	}
	root, err := FindModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	r := RunWeeklyLive(WeeklyLiveOptions{
		ModuleRoot:  root,
		SkipScripts: true,
	})
	if r.DocUITestOK {
		t.Fatal("doc/ui test should be skipped in preflight mode")
	}
	if len(r.Notes) == 0 || !stringsContains(r.Notes, "doc_ui_test_skipped_preflight") {
		t.Fatalf("expected skip note: %+v", r.Notes)
	}
	// preflight likely fails without live node - that's expected on dev machines
	if r.OK {
		t.Log("weekly live passed with skip-scripts (live runner present)")
	}
}

func stringsContains(notes []string, sub string) bool {
	for _, n := range notes {
		if strings.Contains(n, sub) {
			return true
		}
	}
	return false
}

func TestRunLiveSoakSkipScriptsNeedsLiveNode(t *testing.T) {
	if !hasGo() {
		t.Skip("go not in PATH")
	}
	root, err := FindModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	r := RunLiveSoak(LiveSoakOptions{
		ModuleRoot:  root,
		SkipScripts: true,
	})
	if r.DocUITestOK && r.StatefulOK {
		if r.OK {
			t.Log("live soak passed with skip-scripts (live runner present)")
		}
		return
	}
	if len(r.Issues) == 0 {
		t.Fatalf("expected doc/ui, stateful, or preflight issue: %+v", r)
	}
}

func TestRunScriptMissing(t *testing.T) {
	root, err := FindModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	sr := RunScript(root, "nonexistent_script.ps1", nil, nil)
	if sr.OK || sr.Error == "" {
		t.Fatalf("%+v", sr)
	}
}
