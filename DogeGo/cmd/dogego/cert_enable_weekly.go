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

func runCertEnableWeekly(args []string) {
	fs := flag.NewFlagSet("cert enable-weekly", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	weeklyOnly := fs.Bool("weekly-only", false, "set DOGEGO_SCHEDULED_WEEKLY_LIVE only")
	requireWalletDat := fs.Bool("require-wallet-dat", false, "also set DOGEGO_WALLET_DAT_REQUIRED=1 for scheduled live-weekly")
	dryRun := fs.Bool("dry-run", false, "print gh commands without running")
	repo := fs.String("repo", "", "GitHub repo owner/name (default: GITHUB_REPOSITORY or git origin)")
	_ = fs.Parse(args)

	root, _ := findGoModuleRoot()
	er := runner.EnableScheduledLive(runner.EnableWeeklyOptions{
		WeeklyOnly:       *weeklyOnly,
		RequireWalletDat: *requireWalletDat,
		DryRun:           *dryRun,
		Repo:             *repo,
		GitRoot:          root,
	})

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(er)
	} else {
		fmt.Printf("=== Enable DogeGo scheduled live CI (%s) ===\n", er.Repo)
		if er.Doc != "" {
			fmt.Println("DOC:", er.Doc)
		}
		for _, v := range er.Vars {
			fmt.Printf("  %s - %s\n", v.Name, v.Note)
		}
		for _, c := range er.Commands {
			if *dryRun {
				fmt.Println("  DRY:", c)
			}
		}
		for _, n := range er.Notes {
			fmt.Println("NOTE:", n)
		}
		for _, i := range er.Issues {
			fmt.Println("FAIL:", i)
		}
		if er.OK {
			fmt.Println("\nScheduled live CI variables configured.")
			fmt.Println("Next: dogego cert weekly-live -mine-bootstrap -require-wallet-dat (dogego-live runner)")
		} else {
			fmt.Println("\nEnable failed - install gh CLI (gh auth login) or pass -repo owner/name")
			fmt.Println("See docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10)")
		}
	}
	if !er.OK {
		os.Exit(1)
	}
}
