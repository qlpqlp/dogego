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

func runCertProvision(args []string) {
	fs := flag.NewFlagSet("cert provision", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	preflight := fs.Bool("preflight", false, "TCP probe reboottestnet RPC ports (44556 DogeGo, 44555 Core)")
	offlineOnly := fs.Bool("offline", false, "skip live port probes (tools + runner label only)")
	runSetup := fs.Bool("run-setup", false, "run dogego cert setup-parity after checklist (implies live RPC)")
	mineBootstrap := fs.Bool("mine-bootstrap", true, "mine blocks when DogeGo wallet balance is below 1 DOGE (-run-setup)")
	mineBlocks := fs.Int("mine-blocks", 101, "blocks to mine for -mine-bootstrap")
	dogePort := fs.Int("dogego-port", 44556, "DogeGo reboottestnet RPC port for -preflight/-run-setup")
	corePort := fs.Int("core-port", 44555, "Core reboottestnet RPC port for -preflight/-run-setup")
	_ = fs.Parse(args)

	vr := runner.VerifyProvision(runner.ProvisionOptions{
		Preflight:     *preflight || *runSetup,
		OfflineOnly:   *offlineOnly,
		RunSetup:      *runSetup,
		MineBootstrap: *mineBootstrap,
		MineBlocks:    *mineBlocks,
		DogeGoPort:    *dogePort,
		CorePort:      *corePort,
	})

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(vr)
	} else {
		fmt.Printf("=== DogeGo dogego-live runner provision (%d/%d) ===\n", vr.Done, vr.Total)
		for _, row := range vr.Checklist {
			mark := "[ ]"
			if row.Done {
				mark = "[x]"
			}
			fmt.Printf("  %s %d. %s\n", mark, row.Step, row.Item)
		}
		for _, n := range vr.Notes {
			fmt.Println("NOTE:", n)
		}
		for _, w := range vr.Auto {
			fmt.Println("AUTO:", w)
		}
		for _, w := range vr.Warnings {
			fmt.Println("WARN:", w)
		}
		for _, i := range vr.Issues {
			fmt.Println("HINT:", i)
		}
		if vr.Doc != "" {
			fmt.Println("DOC:", vr.Doc)
		}
		if vr.OK {
			fmt.Println("\nProvision checklist passed minimum bar (4+ steps).")
		} else {
			fmt.Println("\nProvision checklist incomplete - see docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10)")
		}
	}
	if !vr.OK {
		os.Exit(1)
	}
}