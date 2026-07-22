// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"os/exec"
	"strings"
	"testing"
)

func TestWorkflow10CLISuggestion(t *testing.T) {
	got := Workflow10CLISuggestion(true, true)
	if !strings.Contains(got, "workflow10") || !strings.Contains(got, "require-wallet-dat") || !strings.Contains(got, "include-live-soak") {
		t.Fatalf("%q", got)
	}
}

func TestRunWorkflow10StopAfterProvisionSkipped(t *testing.T) {
	r := RunWorkflow10(Workflow10Options{
		SkipProvision: true,
		StopAfter:     "provision",
	})
	if r.OK {
		t.Fatalf("expected weekly-live failure without live runner: %+v", r)
	}
	if r.Doc != DogegoLiveWorkflow10Doc {
		t.Fatalf("doc %q", r.Doc)
	}
	found := false
	for _, s := range r.Stages {
		if s.ID == "provision" && s.Skipped {
			found = true
		}
	}
	if !found {
		t.Fatalf("stages %#v", r.Stages)
	}
}

func TestRunWorkflow10EnableGitHubDryRun(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not in PATH")
	}
	r := RunWorkflow10(Workflow10Options{
		EnableGitHub: true,
		GitHubDryRun: true,
		GitHubRepo:   "owner/repo",
		SkipProvision: true,
		SkipScripts:   true,
	})
	if r.EnableWeekly == nil || !r.EnableWeekly.OK {
		t.Fatalf("enable weekly: %+v", r.EnableWeekly)
	}
	if len(r.Issues) == 0 && r.OK {
		t.Log("workflow10 passed with skip-provision (live runner present)")
	}
}
