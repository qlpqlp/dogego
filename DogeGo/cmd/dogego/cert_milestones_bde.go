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

func runCertMilestonesBDE(args []string) {
	fs := flag.NewFlagSet("cert milestones-bde", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	_ = fs.Parse(args)

	root, err := findGoModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if !*jsonOut {
		fmt.Println("=== DogeGo milestones B/D/E offline close ===")
		fmt.Println("Runs code-closeable gates for crash/corruption (B), mempool policy (D), and operator workflow (E).")
		fmt.Println("Full milestone sign-off still needs dogego-live: cert weekly-live + cert live-soak.")
	}

	res := runner.RunMilestonesBDEOffline(runner.MilestonesBDEOptions{ModuleRoot: root})
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		for id, slice := range res.Milestones {
			st := "FAIL"
			if slice.OfflineOK {
				st = "PASS"
			}
			fmt.Printf("\nMilestone %s (%s): offline %s\n", id, slice.ID, st)
			for _, n := range slice.Notes {
				fmt.Printf("  note: %s\n", n)
			}
			for _, p := range slice.LivePending {
				fmt.Printf("  live pending: %s\n", p)
			}
		}
		for _, iss := range res.Issues {
			fmt.Printf("issue: %s\n", iss)
		}
		fmt.Println("\n" + runner.MilestonesBDESummaryLine(res))
	}
	if !res.OK {
		os.Exit(1)
	}
}
