// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"dogego/runner"
)

func runCertWorkflow10(args []string) {
	fs := flag.NewFlagSet("cert workflow10", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	mineBootstrap := fs.Bool("mine-bootstrap", false, "mine blocks when DogeGo wallet balance is below 1 DOGE")
	requireWalletDat := fs.Bool("require-wallet-dat", false, "fail unless DOGEGO_WALLET_DAT probes/imports successfully")
	skipScripts := fs.Bool("skip-scripts", false, "preflight-only weekly-live/live-soak: skip PowerShell gate scripts")
	includeLiveSoak := fs.Bool("include-live-soak", false, "run Milestone B live-soak after weekly-live")
	liveSoakMin := fs.Int("live-soak-min", 0, "live-soak duration minutes (0 = env/default)")
	enableGitHub := fs.Bool("enable-github", false, "run dogego cert enable-weekly first (gh variable set)")
	githubApply := fs.Bool("github-apply", false, "with -enable-github: actually run gh (default is dry-run)")
	githubWeeklyOnly := fs.Bool("github-weekly-only", false, "with -enable-github: set DOGEGO_SCHEDULED_WEEKLY_LIVE only")
	githubRepo := fs.String("github-repo", "", "GitHub repo owner/name for -enable-github")
	skipProvision := fs.Bool("skip-provision", false, "skip provision -preflight -run-setup stage")
	stopAfter := fs.String("stop-after", "", "stop after stage: provision or weekly-live")
	dogePort := fs.Int("dogego-port", 44556, "DogeGo reboottestnet RPC port")
	corePort := fs.Int("core-port", 44555, "Core reboottestnet RPC port")
	dataDir := fs.String("datadir", "dogedata", "reboottestnet data directory name")
	_ = fs.Parse(args)

	root, err := findGoModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	res := runner.RunWorkflow10(runner.Workflow10Options{
		ModuleRoot:       root,
		MineBootstrap:    *mineBootstrap,
		RequireWalletDat: *requireWalletDat,
		SkipScripts:      *skipScripts,
		IncludeLiveSoak:  *includeLiveSoak,
		LiveSoakMin:      *liveSoakMin,
		EnableGitHub:     *enableGitHub,
		GitHubDryRun:     !*githubApply,
		GitHubWeeklyOnly: *githubWeeklyOnly,
		GitHubRepo:       *githubRepo,
		SkipProvision:    *skipProvision,
		StopAfter:        *stopAfter,
		DogeGoPort:       *dogePort,
		CorePort:         *corePort,
		DataDir:          *dataDir,
	})

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		fmt.Println("=== DogeGo workflow 10 (dogego-live) ===")
		if res.Doc != "" {
			fmt.Println("DOC:", res.Doc)
		}
		for _, s := range res.Stages {
			status := "FAIL"
			if s.Skipped {
				status = "SKIP"
			} else if s.OK {
				status = "OK"
			}
			fmt.Printf("STAGE %s: %s\n", status, s.ID)
		}
		for _, n := range res.Notes {
			fmt.Println("NOTE:", n)
		}
		for _, w := range res.Warnings {
			fmt.Println("WARN:", w)
		}
		for _, i := range res.Issues {
			fmt.Println("FAIL:", i)
		}
		if res.OK {
			fmt.Println("\nWorkflow 10 passed.")
		} else {
			fmt.Println("\nWorkflow 10 failed.")
		}
	}
	if !res.OK {
		os.Exit(1)
	}
}
