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
	"strings"

	"dogego/walletimport"
)

func runCertWalletImport(args []string) {
	fs := flag.NewFlagSet("cert wallet-import", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	_ = fs.Parse(args)

	root, err := findGoModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	report := map[string]any{
		"ok":      true,
		"offline": "pending",
	}

	if !*jsonOut {
		fmt.Println("=== DogeGo wallet import certification (offline) ===")
		for _, s := range walletimport.DefaultOfflineSuites() {
			fmt.Printf("\n> go %s\n", strings.Join(s.Args, " "))
		}
		fmt.Println("\n> wallet-migration suites (wallet.dat fixtures)")
	}

	walletimport.SetOutput(os.Stdout, os.Stderr)
	if err := walletimport.RunOffline(root); err != nil {
		report["ok"] = false
		report["offline"] = err.Error()
		emitWalletImportReport(*jsonOut, report)
		os.Exit(1)
	}
	report["offline"] = "passed"

	if !*jsonOut {
		fmt.Println("\nWallet import certification passed.")
	}
	emitWalletImportReport(*jsonOut, report)
}

func emitWalletImportReport(jsonOut bool, report map[string]any) {
	if !jsonOut {
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
}
