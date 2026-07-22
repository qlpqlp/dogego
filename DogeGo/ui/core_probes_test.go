// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"

	"dogego/config"
)

func TestAnnotateRestartResumeSummaryCheckpointWarn(t *testing.T) {
	s := map[string]any{}
	AnnotateRestartResumeSummary(s, "", 1000, 100, true, 0)
	if _, ok := s["dogego_checkpoint_probe"]; ok {
		t.Fatal("expected no checkpoint without chain dir")
	}
}

func TestRunCoreProbesNoInvoke(t *testing.T) {
	b := RunCoreProbes("mainnet", "", "", config.File{}, nil)
	if len(b.RestartResume.Issues) == 0 {
		t.Fatal("expected restart resume issue")
	}
	if !b.Wallet.Skipped {
		t.Fatalf("expected wallet skipped: %+v", b.Wallet)
	}
	if !b.Runner.Skipped || !b.Runner.OK {
		t.Fatalf("expected runner skipped on mainnet: %+v", b.Runner)
	}
	if !b.SetupParity.Skipped || !b.SetupParity.OK {
		t.Fatalf("expected setup parity skipped on mainnet: %+v", b.SetupParity)
	}
	if !b.Workflow10.Skipped || !b.Workflow10.OK {
		t.Fatalf("expected workflow10 skipped on mainnet: %+v", b.Workflow10)
	}
}
