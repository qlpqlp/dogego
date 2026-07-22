// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"strings"
	"testing"

	"dogego/config"
)

func TestProbeCoreIbdConvergenceNoRPCSkipped(t *testing.T) {
	out := ProbeCoreIbdConvergence("mainnet", "", "", config.File{})
	if !out.Skipped || !out.OK {
		t.Fatalf("out=%+v", out)
	}
	if out.Reason != "rpc_not_ready" {
		t.Fatalf("reason=%q", out.Reason)
	}
}

func TestProbeCoreIbdConvergenceHint(t *testing.T) {
	out := ProbeCoreIbdConvergence("mainnet", "", "", config.File{})
	if !strings.Contains(out.Hint, "ibd-convergence") {
		t.Fatalf("hint=%q", out.Hint)
	}
}
