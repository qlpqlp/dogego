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

	"dogego/operational"
)

func runCertOperational(args []string) {
	fs := flag.NewFlagSet("cert operational", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	confPath := fs.String("conf", "", "path to dogecoinconf.json (default: search paths)")
	datadir := fs.String("datadir", "", "override datadir")
	dual := fs.Bool("dual", false, "verify mainnet + reboot testnet dual-run (instances.json or dual conf files)")
	_ = fs.Parse(args)

	if *dual {
		dataDir := strings.TrimSpace(*datadir)
		if dataDir == "" {
			f, _, err := certLoadConfig(*confPath, "")
			if err == nil && strings.TrimSpace(f.DataDir) != "" {
				dataDir = f.DataDir
			}
		}
		dr := operational.VerifyDual(dataDir)
		emitDualOperational(*jsonOut, dr)
		if !dr.OK {
			os.Exit(1)
		}
		return
	}

	f, loadedPath, err := certLoadConfig(*confPath, *datadir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	vr := operational.Verify(f)
	emitOperational(*jsonOut, loadedPath, vr)
	if !vr.OK {
		os.Exit(1)
	}
}

func emitOperational(jsonOut bool, confPath string, vr operational.VerifyResult) {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"ok":        vr.OK,
			"conf_path": confPath,
			"verify":    vr,
		})
		return
	}
	fmt.Println("=== DogeGo operational readiness ===")
	if confPath != "" {
		fmt.Println("config:", confPath)
	}
	fmt.Printf("network=%s role=%s\n", vr.Network, vr.Role)
	for _, c := range vr.Checks {
		tag := strings.ToUpper(c.Status)
		if c.Fix != "" {
			fmt.Printf("%s [%s] %s - %s\n", tag, c.ID, c.Message, c.Fix)
		} else {
			fmt.Printf("%s [%s] %s\n", tag, c.ID, c.Message)
		}
	}
	for _, n := range vr.NextSteps {
		fmt.Println("NEXT:", n)
	}
	if vr.Doc != "" {
		fmt.Println("doc:", vr.Doc)
	}
	if vr.OK {
		fmt.Println("\nOperational preflight passed (warnings may still apply).")
	} else {
		fmt.Println("\nOperational preflight failed.")
	}
}

func emitDualOperational(jsonOut bool, dr operational.DualVerifyResult) {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(dr)
		return
	}
	fmt.Println("=== DogeGo dual-run operational readiness (mainnet + reboot testnet) ===")
	fmt.Println("datadir:", dr.DataDir)
	for _, inst := range dr.Instances {
		fmt.Printf("\n--- %s (%s) webui=%s ---\n", inst.Label, inst.Network, inst.WebUI)
		for _, c := range inst.Verify.Checks {
			fmt.Printf("%s [%s] %s\n", strings.ToUpper(c.Status), c.ID, c.Message)
		}
		if inst.Verify.OK {
			fmt.Println("OK")
		} else {
			fmt.Println("FAIL")
		}
	}
	for _, n := range dr.NextSteps {
		fmt.Println("NEXT:", n)
	}
	if dr.Doc != "" {
		fmt.Println("doc:", dr.Doc)
	}
	if dr.OK {
		fmt.Println("\nDual operational preflight passed.")
	} else {
		fmt.Println("\nDual operational preflight failed.")
	}
}
