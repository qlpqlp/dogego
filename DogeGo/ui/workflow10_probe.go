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

// Workflow10ProbeResult is returned by GET /api/core-workflow10-probe.
type Workflow10ProbeResult struct {
	CheckedAt string                  `json:"checked_at"`
	OK        bool                    `json:"ok"`
	Skipped   bool                    `json:"skipped,omitempty"`
	SkipReason string                 `json:"skip_reason,omitempty"`
	Preflight bool                    `json:"preflight"`
	Result    runner.Workflow10Result `json:"result"`
	CLI       string                  `json:"cli,omitempty"`
	Doc       string                  `json:"doc,omitempty"`
}

// Workflow10ProbeOptions configures workflow 10 web preflight.
type Workflow10ProbeOptions struct {
	RequireWalletDat bool
	SkipProvision    bool
	MineBootstrap    bool
}

// ProbeWorkflow10ForNetwork runs dogego cert workflow10 preflight (-skip-scripts) on reboot testnet.
func ProbeWorkflow10ForNetwork(network string, opts Workflow10ProbeOptions) Workflow10ProbeResult {
	net := network
	if net == "" {
		net = "testnet"
	}
	if !config.IsRebootTestnetNetwork(net) {
		return Workflow10ProbeResult{
			CheckedAt:  time.Now().UTC().Format(time.RFC3339),
			OK:         true,
			Skipped:    true,
			SkipReason: "not reboot testnet (network=" + net + ")",
			Preflight:  true,
			CLI:        runner.Workflow10CLISuggestion(opts.RequireWalletDat, false),
			Doc:        runner.DogegoLiveWorkflow10Doc,
		}
	}
	return probeWorkflow10(opts)
}

func probeWorkflow10(opts Workflow10ProbeOptions) Workflow10ProbeResult {
	root, err := runner.FindModuleRoot()
	if err != nil {
		return Workflow10ProbeResult{
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
			OK:        false,
			Preflight: true,
			Result: runner.Workflow10Result{
				Issues: []string{"module_root_missing"},
				Notes:  []string{err.Error()},
				Doc:    runner.DogegoLiveWorkflow10Doc,
			},
			CLI: runner.Workflow10CLISuggestion(opts.RequireWalletDat, false),
			Doc: runner.DogegoLiveWorkflow10Doc,
		}
	}
	res := runner.RunWorkflow10(runner.Workflow10Options{
		ModuleRoot:       root,
		MineBootstrap:    opts.MineBootstrap,
		RequireWalletDat: opts.RequireWalletDat,
		SkipScripts:      true,
		SkipProvision:    opts.SkipProvision,
	})
	return Workflow10ProbeResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		OK:        res.OK,
		Preflight: true,
		Result:    res,
		CLI:       runner.Workflow10CLISuggestion(opts.RequireWalletDat, false),
		Doc:       runner.DogegoLiveWorkflow10Doc,
	}
}
