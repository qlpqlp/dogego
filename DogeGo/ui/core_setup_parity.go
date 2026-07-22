// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"time"

	"dogego/config"
	"dogego/runner"
)

// SetupParityProbeResult is returned by GET /api/core-setup-parity (read-only Milestone D bootstrap check).
type SetupParityProbeResult struct {
	CheckedAt  string                  `json:"checked_at"`
	OK         bool                    `json:"ok"`
	Skipped    bool                    `json:"skipped,omitempty"`
	SkipReason string                  `json:"skip_reason,omitempty"`
	Setup      runner.SetupParityResult `json:"setup"`
	CLI        string                  `json:"cli,omitempty"`
	Hint       string                  `json:"hint,omitempty"`
}

// ProbeSetupParity checks reboottestnet DogeGo + Core wallet readiness for stateful 24/24 gates (no mining).
func ProbeSetupParity(network string) SetupParityProbeResult {
	net := network
	if net == "" {
		net = "testnet"
	}
	out := SetupParityProbeResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		CLI:       "dogego cert setup-parity -mine-bootstrap",
		Hint:      "Milestone D: preflight + wallet balances before stateful Core compare. Use -mine-bootstrap when wallet is empty.",
	}
	if !config.IsRebootTestnetNetwork(net) {
		out.Skipped = true
		out.SkipReason = "not reboot testnet (network=" + net + ")"
		out.OK = true
		return out
	}
	sp := runner.VerifySetupParity(runner.SetupParityOptions{})
	out.Setup = sp
	out.OK = sp.OK
	return out
}
