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

func runCertWeeklyLive(args []string) {
	fs := flag.NewFlagSet("cert weekly-live", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	mineBootstrap := fs.Bool("mine-bootstrap", false, "mine blocks when DogeGo wallet balance is below 1 DOGE")
	requireWalletDat := fs.Bool("require-wallet-dat", false, "fail unless DOGEGO_WALLET_DAT probes/imports successfully")
	includeLongSoak := fs.Bool("include-long-soak", false, "also run ci_scheduled_corruption_soak.ps1 (multi-hour)")
	skipScripts := fs.Bool("skip-scripts", false, "preflight-only: skip PowerShell Core gate and corruption scripts")
	dogePort := fs.Int("dogego-port", 44556, "DogeGo reboottestnet RPC port")
	corePort := fs.Int("core-port", 44555, "Core reboottestnet RPC port")
	dataDir := fs.String("datadir", "dogedata", "reboottestnet data directory name")
	_ = fs.Parse(args)

	root, err := findGoModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	res := runner.RunWeeklyLive(runner.WeeklyLiveOptions{
		ModuleRoot:       root,
		MineBootstrap:    *mineBootstrap,
		RequireWalletDat: *requireWalletDat,
		IncludeLongSoak:  *includeLongSoak,
		SkipScripts:      *skipScripts,
		DogeGoPort:       *dogePort,
		CorePort:         *corePort,
		DataDir:          *dataDir,
	})

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		fmt.Println("=== DogeGo weekly live CI (dogego-live) ===")
		if res.Doc != "" {
			fmt.Println("DOC:", res.Doc)
		}
		fmt.Printf("doc/ui tests: %v\n", res.DocUITestOK)
		for _, n := range res.Notes {
			fmt.Println("NOTE:", n)
		}
		for _, w := range res.Warnings {
			fmt.Println("WARN:", w)
		}
		for _, i := range res.Issues {
			fmt.Println("FAIL:", i)
		}
		for _, s := range res.ScriptSteps {
			status := "FAIL"
			if s.OK {
				status = "OK"
			} else if s.Skipped {
				status = "SKIP"
			}
			fmt.Printf("SCRIPT %s: %s\n", status, s.Script)
		}
		if res.WalletDatImport != nil {
			fmt.Printf("wallet.dat import: status=%s keys=%d\n", res.WalletDatImport.Status, res.WalletDatImport.KeysImported)
		}
		if res.OK {
			fmt.Println("\nWeekly live CI passed.")
		} else {
			fmt.Println("\nWeekly live CI failed.")
			if res.Doc != "" {
				fmt.Println("DOC:", res.Doc)
			}
		}
	}
	if !res.OK {
		os.Exit(1)
	}
}
