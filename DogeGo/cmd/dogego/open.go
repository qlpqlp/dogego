// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"flag"
	"fmt"
	"os"

	"dogego/config"
	"dogego/desktop"
	"dogego/ui"
)

func runOpen(args []string) {
	fs := flag.NewFlagSet("open", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	urlFlag := fs.String("url", "", "optional dogecoin:// or http(s) dashboard URL")
	_ = fs.Parse(args)
	conf, _ := config.ResolveOpenConfig(*urlFlag)
	raw := *urlFlag
	if raw == "" && fs.NArg() > 0 {
		raw = fs.Arg(0)
	}
	if ui.WasOpenedRecently(raw) {
		return
	}
	var err error
	if raw == "" {
		err = desktop.OpenDashboard(conf)
	} else {
		err = desktop.OpenURL(raw, conf)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
