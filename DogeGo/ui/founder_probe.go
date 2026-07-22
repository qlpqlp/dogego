// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"time"

	"dogego/config"
	"dogego/founder"
)

// FounderProbeResult is returned by GET /api/core-founder-probe.
type FounderProbeResult struct {
	CheckedAt  string               `json:"checked_at"`
	OK         bool                 `json:"ok"`
	Skipped    bool                 `json:"skipped,omitempty"`
	SkipReason string               `json:"skip_reason,omitempty"`
	Verify     founder.VerifyResult `json:"verify,omitempty"`
	CLI        string               `json:"cli,omitempty"`
}

// ProbeFounder checks reboot testnet founder readiness (mirrors dogego cert founder).
func ProbeFounder(network string, conf config.File) FounderProbeResult {
	now := time.Now().UTC().Format(time.RFC3339)
	net := network
	if net == "" {
		net = conf.Network
	}
	if net == "" {
		net = "testnet"
	}
	if !config.IsRebootTestnetNetwork(net) {
		return FounderProbeResult{
			CheckedAt:  now,
			OK:         true,
			Skipped:    true,
			SkipReason: "not reboot testnet (network=" + net + ")",
			CLI:        "dogego cert founder",
		}
	}
	vr := founder.Verify(conf)
	return FounderProbeResult{
		CheckedAt: now,
		OK:        vr.OK,
		Verify:    vr,
		CLI:       "dogego cert founder",
	}
}
