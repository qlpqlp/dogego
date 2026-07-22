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
	"os/exec"
	"strings"
)

func runCertMining(args []string) {
	fs := flag.NewFlagSet("cert mining", flag.ExitOnError)
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
		"hint":    "Live: GET /api/core-mining-probe or scripts/core_mining_workflow.ps1; optional Core GBT compare when tips align",
	}

	suites := []struct {
		Name string
		Args []string
	}{
		{Name: "mining probe + operator cert gate", Args: []string{"test", "./ui", "-run", "TestProbeCoreMining|TestApplyCoreOperatorCertMining|TestOperatorCertWebGateIDs", "-count=1", "-timeout", "120s"}},
		{Name: "GBT + aux mining RPC", Args: []string{"test", "./rpc", "-run", "TestExecGetBlockTemplate|TestWaitGBTLongpoll|TestExecCreateAuxBlock|TestHandlerSubmitAuxBlock|TestHandlerCreateAuxBlock", "-count=1", "-timeout", "120s"}},
	}

	if !*jsonOut {
		fmt.Println("=== DogeGo mining certification (offline) ===")
		for _, s := range suites {
			fmt.Printf("\n> go %s\n", strings.Join(s.Args, " "))
		}
	}

	for _, s := range suites {
		cmd := exec.Command("go", s.Args...)
		cmd.Dir = root
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			report["ok"] = false
			report["offline"] = s.Name + ": " + err.Error()
			emitMiningCertReport(*jsonOut, report)
			os.Exit(1)
		}
	}
	report["offline"] = "passed"
	if !*jsonOut {
		fmt.Println("\nMining certification passed (offline). Live Core GBT compare remains optional via core_rpc_addr / core_mining_workflow.ps1.")
	}
	emitMiningCertReport(*jsonOut, report)
}

func emitMiningCertReport(jsonOut bool, report map[string]any) {
	if !jsonOut {
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
}
