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

// RunnerProbesResult is returned by GET /api/core-runner-probes.
type RunnerProbesResult struct {
	CheckedAt     string                 `json:"checked_at"`
	OK            bool                   `json:"ok"`
	Skipped       bool                   `json:"skipped,omitempty"`
	SkipReason    string                 `json:"skip_reason,omitempty"`
	Provision     runner.VerifyResult    `json:"provision"`
	Preflight     runner.PreflightResult `json:"preflight"`
	CLIProvision  string                 `json:"cli_provision,omitempty"`
	CLIPreflight  string                 `json:"cli_preflight,omitempty"`
	CLIWeekly     string                 `json:"cli_weekly,omitempty"`
	CLIWeeklyLive string                 `json:"cli_weekly_live,omitempty"`
	CLILiveSoak   string                 `json:"cli_live_soak,omitempty"`
	CLIWorkflow10 string                 `json:"cli_workflow10,omitempty"`
	Doc           string                 `json:"doc,omitempty"`
}

// RunnerProbeOptions configures runner readiness probes.
type RunnerProbeOptions struct {
	RequireCore      bool
	RequireWalletDat bool
}

// ProbeRunner runs dogego-live provision checklist + RPC preflight (mirrors dogego cert weekly).
func ProbeRunner(opts RunnerProbeOptions) RunnerProbesResult {
	return probeRunner(opts)
}

// ProbeRunnerForNetwork skips dogego-live checks when not on reboot testnet (operator cert matrix).
func ProbeRunnerForNetwork(network string, opts RunnerProbeOptions) RunnerProbesResult {
	net := network
	if net == "" {
		net = "testnet"
	}
	if !config.IsRebootTestnetNetwork(net) {
		return RunnerProbesResult{
			CheckedAt:     time.Now().UTC().Format(time.RFC3339),
			OK:            true,
			Skipped:       true,
			SkipReason:    "not reboot testnet (network=" + net + ")",
			CLIProvision:  "dogego cert provision -preflight",
			CLIWeekly:     "dogego cert weekly",
			CLIWeeklyLive: "dogego cert weekly-live -mine-bootstrap",
			CLILiveSoak:   "dogego cert live-soak -require-soak-env",
			CLIWorkflow10: runner.Workflow10CLISuggestion(false, false),
			Doc:           runner.DogegoLiveWorkflow10Doc,
		}
	}
	return probeRunner(opts)
}

func probeRunner(opts RunnerProbeOptions) RunnerProbesResult {
	prov := runner.VerifyProvision(runner.ProvisionOptions{Preflight: true})
	pf := runner.RunPreflight(runner.PreflightOptions{
		RequireCore:      opts.RequireCore,
		RequireWalletDat: opts.RequireWalletDat,
		WalletDatImport:  runner.WalletDatImportEnabled(opts.RequireWalletDat),
	})
	ok := prov.OK && pf.OK
	preflightCLI := "dogego cert preflight -require-core"
	weeklyCLI := "dogego cert weekly"
	weeklyLiveCLI := "dogego cert weekly-live -mine-bootstrap"
	liveSoakCLI := "dogego cert live-soak -require-soak-env"
	workflow10CLI := runner.Workflow10CLISuggestion(opts.RequireWalletDat, false)
	if opts.RequireWalletDat {
		preflightCLI += " -require-wallet-dat"
		weeklyCLI += " -require-wallet-dat"
		weeklyLiveCLI += " -require-wallet-dat"
		workflow10CLI = runner.Workflow10CLISuggestion(true, false)
	}
	return RunnerProbesResult{
		CheckedAt:     time.Now().UTC().Format(time.RFC3339),
		OK:            ok,
		Provision:     prov,
		Preflight:     pf,
		CLIProvision:  "dogego cert provision -preflight",
		CLIPreflight:  preflightCLI,
		CLIWeekly:     weeklyCLI,
		CLIWeeklyLive: weeklyLiveCLI,
		CLILiveSoak:   liveSoakCLI,
		CLIWorkflow10: workflow10CLI,
		Doc:           runner.DogegoLiveWorkflow10Doc,
	}
}
