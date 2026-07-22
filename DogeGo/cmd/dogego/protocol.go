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

	"dogego/desktop"
)

func runRegisterURLProtocol(args []string) {
	fs := flag.NewFlagSet("register-url-protocol", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	scheme := fs.String("scheme", desktop.DefaultURLScheme, "URL scheme to register (default dogecoin)")
	unregister := fs.Bool("unregister", false, "remove the per-user protocol handler")
	_ = fs.Parse(args)

	var err error
	if *unregister {
		err = desktop.UnregisterURLScheme(*scheme)
		if err == nil {
			fmt.Fprintf(os.Stdout, "Unregistered %s:// for this user.\n", *scheme)
		}
	} else {
		err = desktop.RegisterURLScheme(*scheme)
		if err == nil {
			fmt.Fprintf(os.Stdout, "Registered %s:// for this user (node dashboard + payment links).\n", *scheme)
			fmt.Fprintf(os.Stdout, "Try: %s://node or %s:ADDRESS?amount=1\n", *scheme, *scheme)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
