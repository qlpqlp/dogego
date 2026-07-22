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

func runCertSetupParity(args []string) {
	fs := flag.NewFlagSet("cert setup-parity", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	mineBootstrap := fs.Bool("mine-bootstrap", false, "mine blocks when DogeGo wallet balance is below 1 DOGE")
	mineBlocks := fs.Int("mine-blocks", 101, "blocks to mine for -mine-bootstrap")
	dogePort := fs.Int("dogego-port", 44556, "DogeGo reboottestnet RPC port")
	corePort := fs.Int("core-port", 44555, "Core reboottestnet RPC port")
	_ = fs.Parse(args)

	res := runner.VerifySetupParity(runner.SetupParityOptions{
		MineBootstrap: *mineBootstrap,
		MineBlocks:    *mineBlocks,
		DogeGoPort:    *dogePort,
		CorePort:      *corePort,
	})

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		fmt.Println("=== Reboottestnet Core parity setup ===")
		if res.Doc != "" {
			fmt.Println("DOC:", res.Doc)
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
		if len(res.EnvExports) > 0 {
			fmt.Println("\nExport for stateful Core gate:")
			for _, e := range res.EnvExports {
				fmt.Println("  export", e)
			}
		}
		if len(res.NextSteps) > 0 {
			fmt.Println("\nNext:")
			for _, s := range res.NextSteps {
				fmt.Println(" ", s)
			}
		}
		if res.OK {
			fmt.Println("\nSetup parity checks passed.")
		} else {
			fmt.Println("\nSetup parity checks failed.")
			if res.Doc != "" {
				fmt.Println("DOC:", res.Doc)
			}
		}
	}
	if !res.OK {
		os.Exit(1)
	}
}
