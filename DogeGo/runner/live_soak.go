// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// LiveSoakOptions configures RunLiveSoak (mirrors ci_milestone_b_full_gate.ps1).
type LiveSoakOptions struct {
	ModuleRoot   string
	DurationMin  int
	SkipScripts  bool
	DogeGoPort   int
	DataDir      string
	Network      string
	Host         string
	RPCTimeout   time.Duration
	RequireSoakEnv bool
}

// LiveSoakResult reports Milestone B multi-hour corruption soak readiness/run.
type LiveSoakResult struct {
	OK          bool              `json:"ok"`
	Doc         string              `json:"doc,omitempty"`
	DocUITestOK bool                `json:"doc_ui_test_ok,omitempty"`
	StatefulOK  bool                `json:"stateful_mempool_test_ok,omitempty"`
	Preflight   PreflightResult     `json:"preflight"`
	Script      *ScriptRunResult    `json:"script,omitempty"`
	Issues      []string            `json:"issues,omitempty"`
	Warnings    []string            `json:"warnings,omitempty"`
	Notes       []string            `json:"notes,omitempty"`
	DurationMin int                 `json:"duration_min,omitempty"`
}

// RunLiveSoak runs Milestone B full gate on dogego-live (reboottestnet; disruptive).
func RunLiveSoak(opts LiveSoakOptions) LiveSoakResult {
	r := LiveSoakResult{
		Doc: DogegoLiveWorkflow10Doc,
	}
	if opts.Network == "" {
		opts.Network = "reboottestnet"
	}
	if opts.Network != "reboottestnet" {
		r.Issues = append(r.Issues, "network_not_reboottestnet")
		return r
	}
	if opts.DataDir == "" {
		opts.DataDir = "dogedata"
	}
	if opts.DogeGoPort == 0 {
		opts.DogeGoPort = 44556
	}
	if opts.Host == "" {
		opts.Host = "127.0.0.1"
	}
	if opts.RPCTimeout <= 0 {
		opts.RPCTimeout = 15 * time.Second
	}
	if opts.RequireSoakEnv && strings.TrimSpace(os.Getenv("DOGEGO_SCHEDULED_LIVE_SOAK")) != "1" {
		r.Warnings = append(r.Warnings, "DOGEGO_SCHEDULED_LIVE_SOAK not set")
	}

	duration := opts.DurationMin
	if duration <= 0 {
		if v := strings.TrimSpace(os.Getenv("DOGEGO_CORRUPTION_LONG_MIN")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				duration = n
			}
		}
	}
	r.DurationMin = duration

	root := strings.TrimSpace(opts.ModuleRoot)
	if root == "" {
		var err error
		root, err = FindModuleRoot()
		if err != nil {
			r.Issues = append(r.Issues, "module_root_missing")
			r.Notes = append(r.Notes, err.Error())
			return r
		}
	}

	if opts.SkipScripts {
		r.Notes = append(r.Notes, "doc_ui_test_skipped_preflight")
	} else if err := RunGoTest(root, "./docs/...", "./ui/...", "-count=1"); err != nil {
		r.Issues = append(r.Issues, "doc_ui_test_failed")
		r.Notes = append(r.Notes, err.Error())
		return r
	} else {
		r.DocUITestOK = true
	}

	if opts.SkipScripts {
		r.Notes = append(r.Notes, "stateful_mempool_test_skipped_preflight")
	} else if err := RunGoTest(root, "./consensus", "-run", "TestStatefulMempool", "-count=1"); err != nil {
		r.Issues = append(r.Issues, "stateful_mempool_test_failed")
		r.Notes = append(r.Notes, err.Error())
		return r
	} else {
		r.StatefulOK = true
	}

	pf := RunPreflight(PreflightOptions{
		RequireCore: true,
		DogeGoPort:  opts.DogeGoPort,
		Host:        opts.Host,
		RPCTimeout:  opts.RPCTimeout,
	})
	r.Preflight = pf
	if !pf.OK {
		r.Issues = append(r.Issues, "preflight_failed")
		r.Issues = append(r.Issues, pf.Issues...)
		return r
	}

	if opts.SkipScripts {
		r.Notes = append(r.Notes, "script_steps_skipped")
		r.OK = true
		return r
	}

	args := []string{
		"-Network", opts.Network,
		"-DataDir", opts.DataDir,
		"-DogeGoRpcPort", strconv.Itoa(opts.DogeGoPort),
	}
	if duration > 0 {
		args = append(args, "-DurationMin", strconv.Itoa(duration))
	}
	_ = os.Setenv("DOGEGO_CORRUPTION_LONG_SOAK", "1")
	if duration > 0 {
		_ = os.Setenv("DOGEGO_CORRUPTION_LONG_MIN", strconv.Itoa(duration))
	}

	sr := RunScript(root, "ci_milestone_b_full_gate.ps1", args, nil)
	r.Script = &sr
	if !sr.OK {
		r.Issues = append(r.Issues, "live_soak_script_failed")
		if sr.Error != "" {
			r.Notes = append(r.Notes, sr.Error)
		}
		return r
	}

	r.OK = true
	return r
}
