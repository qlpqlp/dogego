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
	"strings"

	"dogego/config"
	"dogego/httptls"
)

func runTLS(args []string) {
	if len(args) == 0 {
		tlsUsage()
		os.Exit(2)
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "trust-ca":
		runTLSTrustCA(args[1:])
	case "status":
		runTLSStatus(args[1:])
	default:
		tlsUsage()
		os.Exit(2)
	}
}

func tlsUsage() {
	fmt.Fprintf(os.Stderr, "usage:\n"+
		"  %s tls trust-ca [-datadir DIR]   install DogeGo local CA into user trust store\n"+
		"  %s tls status [-datadir DIR]     show local TLS material paths and trust state\n",
		os.Args[0], os.Args[0])
}

func runTLSTrustCA(args []string) {
	fs := flag.NewFlagSet("tls trust-ca", flag.ExitOnError)
	dataDir := fs.String("datadir", "", "DogeGo base datadir (default from dogecoinconf.json)")
	_ = fs.Parse(args)
	dir := resolveTLSDataDir(*dataDir)
	mat, err := httptls.EnsureLocalMaterial(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	res := httptls.TrustLocalCA(mat.CACertPath)
	if res.Detail != "" {
		fmt.Println(res.Detail)
	}
	if res.Hint != "" {
		fmt.Println(res.Hint)
	}
	if !res.OK {
		os.Exit(1)
	}
}

func runTLSStatus(args []string) {
	fs := flag.NewFlagSet("tls status", flag.ExitOnError)
	dataDir := fs.String("datadir", "", "DogeGo base datadir (default from dogecoinconf.json)")
	_ = fs.Parse(args)
	dir := resolveTLSDataDir(*dataDir)
	mat, err := httptls.EnsureLocalMaterial(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	st := httptls.CATrustStatus(mat.CACertPath)
	fmt.Printf("datadir: %s\n", dir)
	fmt.Printf("ca_cert: %s\n", mat.CACertPath)
	fmt.Printf("ca_key:  %s\n", mat.CAKeyPath)
	fmt.Printf("trusted: %v\n", st.Trusted)
	if st.Detail != "" {
		fmt.Println(st.Detail)
	}
}

func resolveTLSDataDir(flagDir string) string {
	if d := strings.TrimSpace(flagDir); d != "" {
		return d
	}
	f, _ := config.LoadFirst()
	if strings.TrimSpace(f.DataDir) != "" {
		return f.DataDir
	}
	fmt.Fprintln(os.Stderr, "datadir required (-datadir or dogecoinconf.json)")
	os.Exit(2)
	return ""
}
