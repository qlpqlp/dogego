// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"fmt"
	"os"
	"strings"

	"dogego/fieldevidence"
	"dogego/offlinegate"
)

func runCertFieldEvidence(args []string) {
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "usage: dogego cert field-evidence")
		os.Exit(2)
	}
	root, err := findGoModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("=== Mainnet field evidence certification (offline) ===")
	fmt.Println("\n> bootstrap consensus/testdata (canonical)")
	if err := offlinegate.RunBootstrap(root, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "\nFAIL: bootstrap consensus/testdata")
		os.Exit(1)
	}
	for _, s := range fieldevidence.DefaultSuites() {
		fmt.Printf("\n> go %s\n", strings.Join(s.Args, " "))
	}
	if err := fieldevidence.RunOffline(root, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "\nFAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\nMainnet field evidence certification passed.")
}
