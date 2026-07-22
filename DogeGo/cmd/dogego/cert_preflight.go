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

func runCertPreflight(args []string) {
	fs := flag.NewFlagSet("cert preflight", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	offlineOnly := fs.Bool("offline", false, "tools check only (skip live RPC probes)")
	requireCore := fs.Bool("require-core", false, "fail when Core reboottestnet RPC is unreachable")
	requireWalletDat := fs.Bool("require-wallet-dat", false, "fail unless DOGEGO_WALLET_DAT probes/decrypts successfully")
	dogePort := fs.Int("dogego-port", 44556, "DogeGo reboottestnet RPC port")
	corePort := fs.Int("core-port", 44555, "Core reboottestnet RPC port")
	_ = fs.Parse(args)

	pr := runner.RunPreflight(runner.PreflightOptions{
		OfflineOnly:      *offlineOnly,
		RequireCore:      *requireCore,
		RequireWalletDat: *requireWalletDat,
		DogeGoPort:       *dogePort,
		CorePort:         *corePort,
	})

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(pr)
	} else {
		fmt.Println("=== DogeGo CI runner preflight (dogego-live) ===")
		fmt.Println("DOC:", runner.DogegoLiveWorkflow10Doc)
		for _, n := range pr.Notes {
			fmt.Println("NOTE:", n)
		}
		for _, w := range pr.Warnings {
			fmt.Println("WARN:", w)
		}
		for _, i := range pr.Issues {
			fmt.Println("FAIL:", i)
		}
		if pr.WalletDatImport != nil {
			fmt.Printf("  wallet.dat RPC: status=%s keys_imported=%d\n",
				pr.WalletDatImport.Status, pr.WalletDatImport.KeysImported)
			if pr.WalletDatImport.Error != "" {
				fmt.Println("  wallet.dat RPC error:", pr.WalletDatImport.Error)
			}
		}
		if pr.OK {
			fmt.Println("\nCI runner preflight passed.")
		} else {
			fmt.Println("\nCI runner preflight failed - see docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10)")
		}
	}
	if !pr.OK {
		os.Exit(1)
	}
}
