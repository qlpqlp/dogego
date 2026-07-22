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

	"dogego/autostart"
)

func runCertAutostart(args []string) {
	fs := flag.NewFlagSet("cert autostart", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	confFlag := fs.String("conf", "", "path to dogecoinconf.json (default: search paths)")
	_ = fs.Parse(args)

	f, confPath, err := certLoadConfig(*confFlag, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	want := f.AutostartOnLogin()
	vr := autostart.VerifyLogin(want)

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"ok":        vr.OK,
			"conf_path": confPath,
			"autostart": strings.TrimSpace(f.Autostart),
			"verify":    vr,
		})
	} else {
		fmt.Println("=== DogeGo OS autostart check ===")
		if confPath != "" {
			fmt.Println("config:", confPath)
		}
		fmt.Printf("autostart=%q want_login=%v platform=%s\n", strings.TrimSpace(f.Autostart), want, vr.Status.Platform)
		if vr.Status.Method != "" {
			fmt.Println("method:", vr.Status.Method)
		}
		if vr.Status.Detail != "" {
			fmt.Println("detail:", vr.Status.Detail)
		}
		for _, w := range vr.Warnings {
			fmt.Println("WARN:", w)
		}
		for _, n := range vr.Notes {
			fmt.Println("NOTE:", n)
		}
		for _, i := range vr.Issues {
			fmt.Println("FAIL:", i)
		}
		if vr.OK {
			fmt.Println("\nAutostart check passed.")
		} else {
			fmt.Println("\nAutostart check failed - enable in Settings → Interface or re-run setup wizard, then save config.")
		}
	}
	if !vr.OK {
		os.Exit(1)
	}
}
