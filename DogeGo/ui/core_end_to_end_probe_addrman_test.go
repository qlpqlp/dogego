// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"strings"
	"testing"
)

func TestEndToEndFromProbesAddrmanStep(t *testing.T) {
	tried := 3
	newN := 7
	out := EndToEndFromProbes(CoreProbesBundle{
		Maintenance:    CoreMaintenanceResult{OK: true},
		RestartResume:  CoreRestartResumeResult{OK: true},
		IbdConvergence: CoreIbdConvergenceProbeResult{OK: true},
		Addrman: CoreAddrmanProbeResult{
			OK: true, Tried: &tried, NewAddrs: &newN, BucketSchemaOK: true,
		},
		Reindex:       CoreReindexProbeResult{OK: true},
		Bip152:        CoreBip152ProbeResult{OK: true},
		MempoolParity: MempoolParityProbeResult{Skipped: true, Reason: "warming up"},
		SetupParity:   SetupParityProbeResult{Skipped: true, OK: true},
		Wallet:        CoreWalletProbeResult{Skipped: true},
	})
	for _, s := range out.Steps {
		if s.Name == "addrman" && s.OK && strings.Contains(s.Note, "tried=3") {
			return
		}
	}
	t.Fatalf("steps=%+v", out.Steps)
}
