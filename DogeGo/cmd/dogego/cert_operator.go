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

	"dogego/operatorworkflow"
)

func runCertOperator(args []string) {
	fs := flag.NewFlagSet("cert operator", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	skipFieldEvidence := fs.Bool("skip-field-evidence", false, "skip mainnet field-evidence suites")
	skipWalletImport := fs.Bool("skip-wallet-import", false, "skip wallet import certification slice")
	_ = fs.Parse(args)

	root, err := findGoModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	report := map[string]any{
		"ok":             true,
		"core":           "pending",
		"field_evidence": "skipped",
		"wallet_import":  "skipped",
	}
	if !*skipFieldEvidence {
		report["field_evidence"] = "pending"
	}
	if !*skipWalletImport {
		report["wallet_import"] = "pending"
	}

	if !*jsonOut {
		fmt.Println("=== DogeGo operator workflow certification (offline) ===")
		for _, s := range operatorworkflow.DefaultCoreSuites() {
			fmt.Printf("\n> go %s\n", strings.Join(s.Args, " "))
		}
		if !*skipFieldEvidence {
			fmt.Println("\n> field-evidence suites (bootstrap + mainnet field corpus)")
		}
		if !*skipWalletImport {
			fmt.Println("\n> wallet-import suites")
		}
	}

	operatorworkflow.SetOutput(os.Stdout, os.Stderr)
	if err := operatorworkflow.RunOffline(root, *skipFieldEvidence, *skipWalletImport); err != nil {
		report["ok"] = false
		if strings.Contains(err.Error(), "field-evidence:") {
			report["field_evidence"] = err.Error()
		} else if strings.Contains(err.Error(), "wallet-import:") || strings.Contains(err.Error(), "wallet-migration:") {
			report["wallet_import"] = err.Error()
		} else {
			report["core"] = err.Error()
		}
		emitOperatorCertReport(*jsonOut, report)
		os.Exit(1)
	}
	report["core"] = "passed"
	if !*skipFieldEvidence {
		report["field_evidence"] = "passed"
	}
	if !*skipWalletImport {
		report["wallet_import"] = "passed"
	}

	if !*jsonOut {
		fmt.Println("\nOperator workflow certification passed.")
	}
	emitOperatorCertReport(*jsonOut, report)
}

func emitOperatorCertReport(jsonOut bool, report map[string]any) {
	if !jsonOut {
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
}
