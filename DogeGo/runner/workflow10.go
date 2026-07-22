// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"strings"
	"time"
)

// Workflow10Options configures RunWorkflow10 (dogego-live workflow 10 orchestrator).
type Workflow10Options struct {
	ModuleRoot       string
	MineBootstrap    bool
	RequireWalletDat bool
	SkipScripts      bool
	IncludeLiveSoak  bool
	LiveSoakMin      int
	EnableGitHub     bool
	GitHubDryRun     bool
	GitHubWeeklyOnly bool
	GitHubRepo       string
	SkipProvision    bool
	StopAfter        string // "", "provision", "weekly-live"
	DogeGoPort       int
	CorePort         int
	DataDir          string
	Host             string
	RPCTimeout       time.Duration
}

// Workflow10Stage is one workflow 10 step outcome.
type Workflow10Stage struct {
	ID      string `json:"id"`
	OK      bool   `json:"ok"`
	Skipped bool   `json:"skipped,omitempty"`
}

// Workflow10Result reports the full workflow 10 cert chain.
type Workflow10Result struct {
	OK           bool               `json:"ok"`
	Doc          string             `json:"doc,omitempty"`
	Stages       []Workflow10Stage  `json:"stages"`
	EnableWeekly *EnableWeeklyResult `json:"enable_weekly,omitempty"`
	Provision    *VerifyResult      `json:"provision,omitempty"`
	WeeklyLive   *WeeklyLiveResult  `json:"weekly_live,omitempty"`
	LiveSoak     *LiveSoakResult    `json:"live_soak,omitempty"`
	Issues       []string           `json:"issues,omitempty"`
	Warnings     []string           `json:"warnings,omitempty"`
	Notes        []string           `json:"notes,omitempty"`
}

// RunWorkflow10 runs dogego-live workflow 10: optional enable-github → provision → weekly-live → optional live-soak.
func RunWorkflow10(opts Workflow10Options) Workflow10Result {
	r := Workflow10Result{Doc: DogegoLiveWorkflow10Doc}
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
	if opts.DogeGoPort == 0 {
		opts.DogeGoPort = 44556
	}
	if opts.CorePort == 0 {
		opts.CorePort = 44555
	}
	if opts.DataDir == "" {
		opts.DataDir = "dogedata"
	}
	if opts.Host == "" {
		opts.Host = "127.0.0.1"
	}
	if opts.RPCTimeout <= 0 {
		opts.RPCTimeout = 15 * time.Second
	}
	stopAfter := strings.TrimSpace(strings.ToLower(opts.StopAfter))

	if opts.EnableGitHub {
		ew := EnableScheduledLive(EnableWeeklyOptions{
			DryRun:           opts.GitHubDryRun,
			WeeklyOnly:       opts.GitHubWeeklyOnly,
			RequireWalletDat: opts.RequireWalletDat,
			Repo:             opts.GitHubRepo,
			GitRoot:          root,
		})
		r.EnableWeekly = &ew
		r.Stages = append(r.Stages, Workflow10Stage{ID: "enable-github", OK: ew.OK})
		if !ew.OK {
			r.Issues = append(r.Issues, "enable_github_failed")
			return r
		}
	} else {
		r.Stages = append(r.Stages, Workflow10Stage{ID: "enable-github", OK: true, Skipped: true})
	}

	if !opts.SkipProvision {
		prov := VerifyProvision(ProvisionOptions{
			Preflight:     true,
			RunSetup:      true,
			MineBootstrap: opts.MineBootstrap,
			DogeGoPort:    opts.DogeGoPort,
			CorePort:      opts.CorePort,
		})
		r.Provision = &prov
		r.Stages = append(r.Stages, Workflow10Stage{ID: "provision", OK: prov.OK})
		if !prov.OK {
			r.Issues = append(r.Issues, "provision_failed")
			return r
		}
		if stopAfter == "provision" {
			r.OK = true
			r.Notes = append(r.Notes, "stopped after provision")
			return r
		}
	} else {
		r.Stages = append(r.Stages, Workflow10Stage{ID: "provision", OK: true, Skipped: true})
	}

	wl := RunWeeklyLive(WeeklyLiveOptions{
		ModuleRoot:       root,
		MineBootstrap:    opts.MineBootstrap,
		RequireWalletDat: opts.RequireWalletDat,
		SkipScripts:      opts.SkipScripts,
		DogeGoPort:       opts.DogeGoPort,
		CorePort:         opts.CorePort,
		DataDir:          opts.DataDir,
		Host:             opts.Host,
		RPCTimeout:       opts.RPCTimeout,
	})
	r.WeeklyLive = &wl
	r.Stages = append(r.Stages, Workflow10Stage{ID: "weekly-live", OK: wl.OK})
	if !wl.OK {
		r.Issues = append(r.Issues, "weekly_live_failed")
		return r
	}
	if stopAfter == "weekly-live" {
		r.OK = true
		r.Notes = append(r.Notes, "stopped after weekly-live")
		return r
	}

	if opts.IncludeLiveSoak {
		ls := RunLiveSoak(LiveSoakOptions{
			ModuleRoot:     root,
			DurationMin:    opts.LiveSoakMin,
			SkipScripts:    opts.SkipScripts,
			DogeGoPort:     opts.DogeGoPort,
			DataDir:        opts.DataDir,
			Host:           opts.Host,
			RPCTimeout:     opts.RPCTimeout,
			RequireSoakEnv: true,
		})
		r.LiveSoak = &ls
		r.Stages = append(r.Stages, Workflow10Stage{ID: "live-soak", OK: ls.OK})
		if !ls.OK {
			r.Issues = append(r.Issues, "live_soak_failed")
			return r
		}
	} else {
		r.Stages = append(r.Stages, Workflow10Stage{ID: "live-soak", OK: true, Skipped: true})
	}

	r.OK = true
	r.Notes = append(r.Notes, "workflow 10 sequence complete")
	return r
}

// Workflow10CLISuggestion returns the recommended workflow 10 command for operators.
func Workflow10CLISuggestion(requireWalletDat, includeLiveSoak bool) string {
	cmd := "dogego cert workflow10 -mine-bootstrap"
	if requireWalletDat {
		cmd += " -require-wallet-dat"
	}
	if includeLiveSoak {
		cmd += " -include-live-soak"
	}
	return cmd
}
