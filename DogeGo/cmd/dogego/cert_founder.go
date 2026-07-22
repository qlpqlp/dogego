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

	"dogego/founder"
)

func runCertFounder(args []string) {
	fs := flag.NewFlagSet("cert founder", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	confPath := fs.String("conf", "", "path to dogecoinconf.json (default: search paths)")
	datadir := fs.String("datadir", "", "override datadir for preflight (default: from config)")
	_ = fs.Parse(args)

	f, loadedPath, err := certLoadConfig(*confPath, *datadir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if loadedPath == "" && f.Network == "" {
		f.Network = "testnet"
	}
	vr := founder.Verify(f)

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"ok":        vr.OK,
			"conf_path": loadedPath,
			"datadir":   strings.TrimSpace(*datadir),
			"verify":    vr,
		})
	} else {
		fmt.Println("=== DogeGo reboot testnet founder preflight ===")
		if loadedPath != "" {
			fmt.Println("config:", loadedPath)
		} else {
			fmt.Println("config: (none found - checking testnet defaults)")
		}
		if d := strings.TrimSpace(*datadir); d != "" {
			fmt.Println("datadir override:", d)
		}
		fmt.Printf("network=%s datadir=%s p2p_port=%d\n", vr.Network, vr.DataDir, vr.P2PPort)
		for _, c := range vr.Checks {
			tag := strings.ToUpper(c.Status)
			if c.Fix != "" {
				fmt.Printf("%s [%s] %s - %s\n", tag, c.ID, c.Message, c.Fix)
			} else {
				fmt.Printf("%s [%s] %s\n", tag, c.ID, c.Message)
			}
		}
		for _, n := range vr.Notes {
			fmt.Println("NOTE:", n)
		}
		if vr.Doc != "" {
			fmt.Println("doc:", vr.Doc)
		}
		if vr.OK {
			fmt.Println("\nFounder preflight passed (warnings may still apply).")
		} else {
			fmt.Println("\nFounder preflight failed - fix issues above before sharing addnode with joiners.")
		}
	}
	if !vr.OK {
		os.Exit(1)
	}
}
