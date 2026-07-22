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

func runCertLiveSoak(args []string) {
	fs := flag.NewFlagSet("cert live-soak", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	durationMin := fs.Int("duration-min", 0, "corruption soak duration in minutes (default from DOGEGO_CORRUPTION_LONG_MIN or 45)")
	requireSoakEnv := fs.Bool("require-soak-env", false, "warn unless DOGEGO_SCHEDULED_LIVE_SOAK=1")
	skipScripts := fs.Bool("skip-scripts", false, "preflight-only: skip ci_milestone_b_full_gate.ps1 soak script")
	dogePort := fs.Int("dogego-port", 44556, "DogeGo reboottestnet RPC port")
	dataDir := fs.String("datadir", "dogedata", "reboottestnet data directory name")
	_ = fs.Parse(args)

	root, err := findGoModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	res := runner.RunLiveSoak(runner.LiveSoakOptions{
		ModuleRoot:     root,
		DurationMin:    *durationMin,
		RequireSoakEnv: *requireSoakEnv,
		SkipScripts:    *skipScripts,
		DogeGoPort:     *dogePort,
		DataDir:        *dataDir,
	})

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		fmt.Println("=== DogeGo live corruption soak (Milestone B full) ===")
		if res.Doc != "" {
			fmt.Println("DOC:", res.Doc)
		}
		if res.DurationMin > 0 {
			fmt.Printf("duration_min=%d\n", res.DurationMin)
		}
		fmt.Printf("doc/ui tests: %v stateful_mempool: %v\n", res.DocUITestOK, res.StatefulOK)
		for _, n := range res.Notes {
			fmt.Println("NOTE:", n)
		}
		for _, w := range res.Warnings {
			fmt.Println("WARN:", w)
		}
		for _, i := range res.Issues {
			fmt.Println("FAIL:", i)
		}
		if res.Script != nil {
			status := "FAIL"
			if res.Script.OK {
				status = "OK"
			}
			fmt.Printf("SCRIPT %s: %s\n", status, res.Script.Script)
		}
		if res.OK {
			fmt.Println("\nLive soak passed.")
		} else {
			fmt.Println("\nLive soak failed.")
			if res.Doc != "" {
				fmt.Println("DOC:", res.Doc)
			}
		}
	}
	if !res.OK {
		os.Exit(1)
	}
}
