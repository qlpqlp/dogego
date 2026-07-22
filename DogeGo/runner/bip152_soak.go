// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// Bip152SoakOptions configures RunBip152Soak.
type Bip152SoakOptions struct {
	ModuleRoot     string
	SkipLive       bool
	DurationMin    int
	IntervalSec    int
	RequireRelay   bool
	RpcPort        int
	RequireLiveEnv bool
}

// Bip152SoakResult reports offline AuxPoW/cmpct edges + optional live timed soak.
type Bip152SoakResult struct {
	OK          bool             `json:"ok"`
	WireTestOK  bool             `json:"wire_cmpct_test_ok,omitempty"`
	NodeTestOK  bool             `json:"node_cmpct_test_ok,omitempty"`
	RPCTestOK   bool             `json:"rpc_cmpct_test_ok,omitempty"`
	UITestOK    bool             `json:"ui_bip152_probe_test_ok,omitempty"`
	Script      *ScriptRunResult `json:"script,omitempty"`
	Issues      []string         `json:"issues,omitempty"`
	Warnings    []string         `json:"warnings,omitempty"`
	Notes       []string         `json:"notes,omitempty"`
	DurationMin int              `json:"duration_min,omitempty"`
	SkipLive    bool             `json:"skip_live"`
}

// RunBip152Soak runs offline BIP152 AuxPoW/cmpct edge tests and optionally the live PS1 soak gate.
// Offline is the default CI path (SkipLive true). Live soak requires SkipLive false on Windows with PowerShell.
func RunBip152Soak(opts Bip152SoakOptions) Bip152SoakResult {
	r := Bip152SoakResult{SkipLive: opts.SkipLive}
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

	if err := RunGoTest(root, "./wire", "-run", "TestBuildHeaderAndShortIDsFromBlock_rejectsAuxpow|TestReconstructBlockFromCmpct_rejectsAuxpow", "-count=1"); err != nil {
		r.Issues = append(r.Issues, "wire_cmpct_auxpow_test_failed")
		r.Notes = append(r.Notes, err.Error())
		return r
	}
	r.WireTestOK = true

	if err := RunGoTest(root, "./node", "-run", "TestNegotiateSendCmpct_|TestCmpctHBSessionCounts|TestAnnounceBlockHash_auxpow|TestHandleInboundCmpctBlock_|TestNoteCmpctWireIgnored_|TestCmpctInbound|TestHandleInboundGetData_CmpctBlock", "-count=1"); err != nil {
		r.Issues = append(r.Issues, "node_cmpct_test_failed")
		r.Notes = append(r.Notes, err.Error())
		return r
	}
	r.NodeTestOK = true

	if err := RunGoTest(root, "./rpc", "-run", "TestDogegoCmpctRelayCounterKeys", "-count=1"); err != nil {
		r.Issues = append(r.Issues, "rpc_cmpct_keys_test_failed")
		r.Notes = append(r.Notes, err.Error())
		return r
	}
	r.RPCTestOK = true

	if err := RunGoTest(root, "./ui", "-run", "TestProbeCoreBip152|TestParseGetPeerInfoBip152|TestAnnotateCmpctRelay|TestAnnotateBip152", "-count=1"); err != nil {
		r.Issues = append(r.Issues, "ui_bip152_probe_test_failed")
		r.Notes = append(r.Notes, err.Error())
		return r
	}
	r.UITestOK = true

	r.Notes = append(r.Notes, "offline_auxpow_cmpct_edges_ok")
	r.Notes = append(r.Notes, "Go ProbeCoreBip152 is soft on peers-without-HB (notes only); PS1 live soak may require HB when not IBD")

	if opts.SkipLive {
		r.Notes = append(r.Notes, "live_soak_skipped")
		r.OK = true
		return r
	}

	if opts.RequireLiveEnv && strings.TrimSpace(os.Getenv("DOGEGO_BIP152_LIVE_SOAK")) != "1" {
		r.Warnings = append(r.Warnings, "DOGEGO_BIP152_LIVE_SOAK not set")
	}

	if runtime.GOOS != "windows" {
		r.Warnings = append(r.Warnings, "live_soak_windows_only")
		r.Notes = append(r.Notes, "live_soak_skipped_non_windows")
		r.OK = true
		return r
	}

	duration := opts.DurationMin
	if duration <= 0 {
		if v := strings.TrimSpace(os.Getenv("DOGEGO_BIP152_SOAK_MIN")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				duration = n
			}
		}
	}
	if duration <= 0 {
		duration = 15
	}
	r.DurationMin = duration

	interval := opts.IntervalSec
	if interval <= 0 {
		if v := strings.TrimSpace(os.Getenv("DOGEGO_BIP152_SOAK_INTERVAL")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 30 {
				interval = n
			}
		}
	}
	if interval <= 0 {
		interval = 60
	}

	args := []string{
		"-DurationMin", strconv.Itoa(duration),
		"-IntervalSec", strconv.Itoa(interval),
	}
	if opts.RpcPort > 0 {
		args = append(args, "-RpcPort", strconv.Itoa(opts.RpcPort))
	}
	requireRelay := opts.RequireRelay || strings.TrimSpace(os.Getenv("DOGEGO_BIP152_SOAK_REQUIRE_RELAY")) == "1"
	if requireRelay {
		args = append(args, "-RequireRelayActivity")
	}
	_ = os.Setenv("DOGEGO_BIP152_LIVE_SOAK", "1")

	sr := RunScript(root, "bip152_live_soak_gate.ps1", args, nil)
	r.Script = &sr
	if !sr.OK {
		r.Issues = append(r.Issues, "bip152_live_soak_script_failed")
		if sr.Error != "" {
			r.Notes = append(r.Notes, sr.Error)
		}
		return r
	}

	r.OK = true
	return r
}
