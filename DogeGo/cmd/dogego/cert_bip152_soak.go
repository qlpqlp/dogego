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

func runCertBip152Soak(args []string) {
	fs := flag.NewFlagSet("cert bip152-soak", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	skipLive := fs.Bool("skip-live", true, "offline AuxPoW/cmpct edges only (default); set false to run live PS1 soak on Windows")
	durationMin := fs.Int("duration-min", 0, "live soak duration minutes (default DOGEGO_BIP152_SOAK_MIN or 15)")
	intervalSec := fs.Int("interval-sec", 0, "live soak probe interval seconds (default 60)")
	requireRelay := fs.Bool("require-relay", false, "live soak: require cmpct relay activity")
	requireLiveEnv := fs.Bool("require-live-env", false, "warn unless DOGEGO_BIP152_LIVE_SOAK=1")
	rpcPort := fs.Int("rpc-port", 0, "live soak RPC port (0 = script default)")
	_ = fs.Parse(args)

	root, err := findGoModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	res := runner.RunBip152Soak(runner.Bip152SoakOptions{
		ModuleRoot:     root,
		SkipLive:       *skipLive,
		DurationMin:    *durationMin,
		IntervalSec:    *intervalSec,
		RequireRelay:   *requireRelay,
		RequireLiveEnv: *requireLiveEnv,
		RpcPort:        *rpcPort,
	})

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		fmt.Println("=== DogeGo BIP152 soak (offline AuxPoW/cmpct + optional live) ===")
		fmt.Printf("wire=%v node=%v rpc=%v ui=%v skip_live=%v\n", res.WireTestOK, res.NodeTestOK, res.RPCTestOK, res.UITestOK, res.SkipLive)
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
			fmt.Println("\nBIP152 soak passed.")
		} else {
			fmt.Println("\nBIP152 soak failed.")
		}
	}
	if !res.OK {
		os.Exit(1)
	}
}
