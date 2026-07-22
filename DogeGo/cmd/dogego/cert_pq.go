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

	"dogego/pqcert"
)

func runCertPQ(args []string) {
	fs := flag.NewFlagSet("cert pq", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	_ = fs.Parse(args)

	root, err := findGoModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	report := map[string]any{
		"ok":      true,
		"scope":   "format_and_carrier_only_no_production_pq_safety",
		"offline": "pending",
	}

	if !*jsonOut {
		fmt.Println("=== DogeGo PQ certification (offline; format/carrier only) ===")
		for _, s := range pqcert.DefaultSuites() {
			fmt.Printf("\n> go %s\n", strings.Join(s.Args, " "))
		}
		fmt.Println("\nNote: this does not certify production PQ safety or consensus relay policy.")
	}

	pqcert.SetOutput(os.Stdout, os.Stderr)
	if err := pqcert.RunOffline(root); err != nil {
		report["ok"] = false
		report["offline"] = err.Error()
		emitPQCertReport(*jsonOut, report)
		os.Exit(1)
	}
	report["offline"] = "passed"

	if !*jsonOut {
		fmt.Println("\nPQ certification passed (format/carrier only; no production PQ safety claim).")
	}
	emitPQCertReport(*jsonOut, report)
}

func emitPQCertReport(jsonOut bool, report map[string]any) {
	if !jsonOut {
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
}
