// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"dogego/walletmigration"
)

// WeeklyLiveOptions configures RunWeeklyLive (mirrors ci_scheduled_weekly_live.ps1).
type WeeklyLiveOptions struct {
	ModuleRoot       string
	MineBootstrap    bool
	RequireWalletDat bool
	IncludeLongSoak  bool
	SkipScripts      bool
	DogeGoPort       int
	CorePort         int
	DataDir          string
	Network          string
	Host             string
	RPCTimeout       time.Duration
}

// WeeklyLiveResult reports the scheduled weekly live CI bundle.
type WeeklyLiveResult struct {
	OK              bool                              `json:"ok"`
	Doc             string                            `json:"doc,omitempty"`
	DocUITestOK     bool                              `json:"doc_ui_test_ok,omitempty"`
	Preflight       PreflightResult                   `json:"preflight"`
	SetupParity     SetupParityResult                 `json:"setup_parity"`
	WalletDatImport *walletmigration.LiveImportResult `json:"wallet_dat_import,omitempty"`
	ScriptSteps     []ScriptRunResult                 `json:"script_steps,omitempty"`
	Issues          []string                          `json:"issues,omitempty"`
	Warnings        []string                          `json:"warnings,omitempty"`
	Notes           []string                          `json:"notes,omitempty"`
}

// RunWeeklyLive runs the dogego-live weekly CI bundle (reboottestnet only).
func RunWeeklyLive(opts WeeklyLiveOptions) WeeklyLiveResult {
	r := WeeklyLiveResult{
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
	if opts.CorePort == 0 {
		opts.CorePort = 44555
	}
	if opts.Host == "" {
		opts.Host = "127.0.0.1"
	}
	if opts.RPCTimeout <= 0 {
		opts.RPCTimeout = 15 * time.Second
	}
	requireWalletDat := opts.RequireWalletDat || strings.TrimSpace(os.Getenv("DOGEGO_WALLET_DAT_REQUIRED")) == "1"
	includeLongSoak := opts.IncludeLongSoak ||
		strings.TrimSpace(os.Getenv("DOGEGO_CORRUPTION_LONG_SOAK")) == "1" ||
		strings.TrimSpace(os.Getenv("DOGEGO_SCHEDULED_LIVE_SOAK")) == "1"

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

	pf := RunPreflight(PreflightOptions{
		RequireCore:      true,
		RequireWalletDat: requireWalletDat,
		WalletDatImport:  false,
		DogeGoPort:       opts.DogeGoPort,
		CorePort:         opts.CorePort,
		Host:             opts.Host,
		RPCTimeout:       opts.RPCTimeout,
	})
	r.Preflight = pf
	if !pf.OK {
		r.Issues = append(r.Issues, "preflight_failed")
		r.Issues = append(r.Issues, pf.Issues...)
		return r
	}

	sp := VerifySetupParity(SetupParityOptions{
		MineBootstrap: opts.MineBootstrap,
		DogeGoPort:    opts.DogeGoPort,
		CorePort:      opts.CorePort,
		Host:          opts.Host,
		RPCTimeout:    opts.RPCTimeout,
	})
	r.SetupParity = sp
	ApplySetupParityEnv(sp.EnvExports)
	_ = os.Setenv("DOGEGO_CORE_COMPARE_MIN", "24")
	if !sp.OK {
		r.Issues = append(r.Issues, "setup_parity_failed")
		r.Issues = append(r.Issues, sp.Issues...)
		return r
	}
	r.Notes = append(r.Notes, sp.Notes...)

	if WalletDatLiveImportNeeded(requireWalletDat) {
		live, err := ImportWalletDatLive(opts.Host, opts.DogeGoPort, requireWalletDat)
		r.WalletDatImport = live
		if err != nil {
			r.Issues = append(r.Issues, "wallet_dat_import_failed")
			r.Notes = append(r.Notes, err.Error())
			return r
		}
		if live != nil {
			r.Notes = append(r.Notes, fmt.Sprintf("wallet_dat_import=%s keys=%d", live.Status, live.KeysImported))
			if live.PoolIndicesReplayed != nil {
				r.Notes = append(r.Notes, fmt.Sprintf("wallet_dat_pool_indices_replayed=%v", *live.PoolIndicesReplayed))
			}
			if live.KeypoolRefillSize != nil && *live.KeypoolRefillSize > 0 {
				r.Notes = append(r.Notes, fmt.Sprintf("wallet_dat_keypool_refill_size=%d", *live.KeypoolRefillSize))
			}
			if live.PoolUnmatchedHint != "" {
				r.Notes = append(r.Notes, "wallet_dat_pool_unmatched_hint="+live.PoolUnmatchedHint)
			}
		}
	}

	if opts.SkipScripts {
		r.Notes = append(r.Notes, "script_steps_skipped")
		r.OK = true
		return r
	}

	scriptArgs := []string{"-DogeGoRpcPort", strconv.Itoa(opts.DogeGoPort)}
	steps := []struct {
		script string
		args   []string
	}{
		{"core_reboottestnet_core_aligned_gate.ps1", scriptArgs},
		{"corruption_extended_cert_mini.ps1", []string{
			"-Network", opts.Network,
			"-DataDir", opts.DataDir,
		}},
	}
	if includeLongSoak {
		steps = append(steps, struct {
			script string
			args   []string
		}{"ci_scheduled_corruption_soak.ps1", []string{
			"-Network", opts.Network,
			"-DataDir", opts.DataDir,
		}})
	}

	for _, step := range steps {
		sr := RunScript(root, step.script, step.args, nil)
		r.ScriptSteps = append(r.ScriptSteps, sr)
		if !sr.OK {
			r.Issues = append(r.Issues, "script_failed:"+step.script)
			if sr.Error != "" {
				r.Notes = append(r.Notes, step.script+": "+sr.Error)
			}
			return r
		}
	}

	r.OK = true
	r.Notes = append(r.Notes, fmt.Sprintf("weekly_live_passed long_soak=%v", includeLongSoak))
	return r
}
