// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "testing"

func TestProbeWorkflow10SkippedOnMainnet(t *testing.T) {
	r := ProbeWorkflow10ForNetwork("mainnet", Workflow10ProbeOptions{})
	if !r.Skipped || !r.OK {
		t.Fatalf("%+v", r)
	}
	if r.CLI == "" || r.Doc == "" {
		t.Fatalf("cli/doc missing: %+v", r)
	}
}

func TestProbeWorkflow10RebootTestnetPreflight(t *testing.T) {
	r := ProbeWorkflow10ForNetwork("reboottestnet", Workflow10ProbeOptions{SkipProvision: true})
	if r.Skipped {
		t.Fatalf("unexpected skip: %+v", r)
	}
	if len(r.Result.Stages) == 0 {
		t.Fatalf("expected stages: %+v", r.Result)
	}
}
