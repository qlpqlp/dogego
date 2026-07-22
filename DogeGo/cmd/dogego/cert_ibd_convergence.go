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

	"dogego/ibdconvergence"
	"dogego/walletmigration"
)

func runCertIBDConvergence(args []string) {
	fs := flag.NewFlagSet("cert ibd-convergence", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	confPath := fs.String("conf", "", "path to dogecoinconf.json (default: search paths)")
	datadir := fs.String("datadir", "", "override datadir (default: from config)")
	network := fs.String("network", "", "network slug (default: from config or mainnet)")
	interval := fs.Int("interval-sec", 120, "seconds between progress snapshots")
	minContig := fs.Int64("min-contiguous", 1, "minimum contiguous raw advance")
	minBlocks := fs.Int64("min-blocks", 1, "minimum chainActive blocks advance")
	minProbe := fs.Int64("min-raw-probe", 1, "minimum rawblocks_sync probe advance")
	maxRegress := fs.Int64("max-contiguous-regression", 64, "fail when contiguous drops more than this")
	diskOnly := fs.Bool("disk-only", false, "skip RPC/web; use on-disk checkpoints only")
	rpcHost := fs.String("rpc-host", "", "RPC host (default: 127.0.0.1 or DOGEGO_RPC_URI)")
	rpcPort := fs.Int("rpc-port", 0, "RPC port (default: from config or 44556)")
	rpcUser := fs.String("rpc-user", "", "RPC user (default: config or DOGEGO_RPC_USER)")
	rpcPass := fs.String("rpc-password", "", "RPC password (default: config or DOGEGO_RPC_PASS)")
	webURL := fs.String("web-url", "", "web dashboard base URL for /api/summary fallback")
	_ = fs.Parse(args)

	f, loadedPath, err := certLoadConfig(*confPath, *datadir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	net := strings.TrimSpace(*network)
	if net == "" {
		net = strings.TrimSpace(f.Network)
	}
	if net == "" {
		net = "mainnet"
	}
	dataDir := strings.TrimSpace(*datadir)
	if dataDir == "" {
		dataDir = strings.TrimSpace(f.DataDir)
	}

	client := walletmigration.DefaultRPCClient()
	if *rpcHost != "" {
		host := strings.TrimSpace(*rpcHost)
		port := *rpcPort
		if port <= 0 {
			port = 44556
		}
		client = walletmigration.RPCClientForHostPort(host, port)
	} else if *rpcPort > 0 {
		client = walletmigration.RPCClientForHostPort("127.0.0.1", *rpcPort)
	}
	if *rpcUser != "" {
		client.User = *rpcUser
	} else if f.RpcUser != "" {
		client.User = f.RpcUser
	}
	if *rpcPass != "" {
		client.Pass = *rpcPass
	} else if f.RpcPassword != "" {
		client.Pass = f.RpcPassword
	}

	opts := ibdconvergence.Options{
		IntervalSec:             *interval,
		MinContiguousAdvance:      *minContig,
		MinBlocksAdvance:          *minBlocks,
		MinRawProbeAdvance:        *minProbe,
		MaxContiguousRegression:   *maxRegress,
		DiskOnly:                  *diskOnly,
		DataDir:                   dataDir,
		Network:                   net,
		RPC:                       client,
		WebURL:                    strings.TrimSpace(*webURL),
	}
	vr := ibdconvergence.Verify(opts)

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"ok":        vr.OK,
			"conf_path": loadedPath,
			"network":   net,
			"datadir":   dataDir,
			"interval_sec": *interval,
			"verify":    vr,
		})
	} else {
		fmt.Println("=== DogeGo IBD convergence check ===")
		if loadedPath != "" {
			fmt.Println("config:", loadedPath)
		}
		fmt.Printf("Interval: %ds  thresholds: contiguous+>=%d blocks+>=%d raw_probe+>=%d\n",
			*interval, *minContig, *minBlocks, *minProbe)
		fmt.Println("T0:", vr.T0.FormatLine())
		fmt.Println("T1:", vr.T1.FormatLine())
		fmt.Printf("Advance: contiguous=+%d blocks=+%d raw_probe=+%d\n", vr.ContiguousAdvance, vr.BlockAdvance, vr.ProbeAdvance)
		for _, n := range vr.Notes {
			fmt.Println("NOTE:", n)
		}
		if vr.ConnectRatePerMin > 0 {
			fmt.Printf("Implied connect rate: ~%.1f blocks/min\n", vr.ConnectRatePerMin)
		}
		for _, i := range vr.Issues {
			fmt.Println("FAIL:", i)
		}
		if vr.OK {
			fmt.Println("OK: IBD forward progress confirmed.")
			if vr.BodyCoveragePct > 0 {
				fmt.Printf("Body coverage: %.2f%%\n", vr.BodyCoveragePct)
			}
		} else {
			fmt.Println("Check Web UI or run: dogego cert ibd-convergence -json")
		}
	}
	if !vr.OK {
		os.Exit(1)
	}
}
