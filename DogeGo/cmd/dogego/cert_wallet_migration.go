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

	"dogego/runner"
	"dogego/wallet/corewallet"
	"dogego/walletmigration"
)

func runCertWalletMigration(args []string) {
	fs := flag.NewFlagSet("cert wallet-migration", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	walletDat := fs.String("wallet-dat", "", "optional path to Core wallet.dat for live file probe")
	passphrase := fs.String("passphrase", "", "wallet passphrase for encrypted wallet.dat (with -wallet-dat)")
	network := fs.String("network", "mainnet", "network for WIF version (mainnet or testnet)")
	skipOffline := fs.Bool("skip-offline", false, "skip go test offline gates")
	offlineOnly := fs.Bool("offline-only", false, "run offline suites only (no file or RPC probe)")
	liveImport := fs.Bool("live-import", false, "probe/import via running DogeGo JSON-RPC (dogego-live)")
	liveProbe := fs.Bool("live-probe", false, "probe only via running DogeGo JSON-RPC (no import)")
	requireWalletDat := fs.Bool("require-wallet-dat", false, "fail unless wallet.dat probes/imports successfully")
	_ = fs.Parse(args)
	if *liveImport && *liveProbe {
		fmt.Fprintln(os.Stderr, "use only one of -live-import or -live-probe")
		os.Exit(2)
	}

	root, err := findGoModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	report := map[string]any{
		"ok":          true,
		"offline":     "skipped",
		"live_probe":  "skipped",
		"live_import": "skipped",
	}

	if !*skipOffline {
		fmt.Println("=== DogeGo wallet.dat migration certification (offline) ===")
		for _, s := range walletmigration.DefaultOfflineSuites() {
			fmt.Printf("\n> go %s\n", strings.Join(s.Args, " "))
		}
		walletmigration.SetOutput(os.Stdout, os.Stderr)
		if err := walletmigration.RunOffline(root); err != nil {
			report["ok"] = false
			report["offline"] = err.Error()
			emitWalletMigrationReport(*jsonOut, report)
			os.Exit(1)
		}
		report["offline"] = "passed"
	}

	if *offlineOnly {
		if !*jsonOut {
			fmt.Println("\nWallet migration certification passed (offline only).")
		}
		emitWalletMigrationReport(*jsonOut, report)
		return
	}

	path, configured := runner.ResolveWalletDatPathConfigured(*walletDat)
	if path == "" {
		if auto := runner.ResolveWalletDatPath(""); auto != "" {
			path = auto
			configured = false
		}
	}
	pass := *passphrase
	if pass == "" {
		pass = os.Getenv("DOGEGO_WALLET_DAT_PASSPHRASE")
	}

	require := *requireWalletDat || strings.TrimSpace(os.Getenv("DOGEGO_WALLET_DAT_REQUIRED")) == "1"

	if path == "" {
		if require {
			report["ok"] = false
			report["live_probe"] = "required_missing"
			emitWalletMigrationReport(*jsonOut, report)
			os.Exit(1)
		}
	} else if *liveImport || *liveProbe {
		client := walletmigration.DefaultRPCClient()
		var live *walletmigration.LiveImportResult
		if *liveImport {
			live, _ = walletmigration.LiveImportViaRPC(client, path, pass)
		} else {
			live, _ = walletmigration.LiveProbeViaRPC(client, path)
		}
		report["live_probe"] = live.Probe
		report["live_import"] = live
		if live.Probe == nil || !live.Probe.IsBDB {
			report["ok"] = false
		}
		if *liveImport {
			if !walletmigration.LiveImportOK(live, require) {
				report["ok"] = false
			}
		} else if !walletmigration.LiveProbeOK(live, require, false) {
			if require && pass != "" {
				if fileLive, err := walletmigration.ProbeFile(path, pass, *network); err != nil || !fileLive.ExtractOK {
					report["ok"] = false
				}
			} else {
				report["ok"] = false
			}
		}
		if !*jsonOut {
			printLiveImportSummary(path, live)
		}
	} else {
		live, err := walletmigration.ProbeFile(path, pass, *network)
		if err != nil {
			if walletmigration.WalletDatProbeOptional(configured, require) {
				report["live_probe"] = "skipped_auto_discover: " + err.Error()
			} else {
				report["ok"] = false
				report["live_probe"] = err.Error()
				emitWalletMigrationReport(*jsonOut, report)
				os.Exit(1)
			}
		} else {
			report["live_probe"] = live
			if live.Probe != nil && !live.Probe.IsBDB {
				if !walletmigration.WalletDatProbeOptional(configured, require) {
					report["ok"] = false
				}
			}
			if pass != "" && !live.ExtractOK {
				report["ok"] = false
			}
			if live.Probe != nil && live.Probe.CanImport && !live.Probe.NeedsPassphrase && !live.ExtractOK {
				report["ok"] = false
			}
			if !*jsonOut {
				printLiveProbeSummary(path, live, pass)
			}
		}
	}

	if report["ok"] != true {
		emitWalletMigrationReport(*jsonOut, report)
		os.Exit(1)
	}
	if !*jsonOut {
		fmt.Println("\nWallet migration certification passed.")
	}
	emitWalletMigrationReport(*jsonOut, report)
}

func printLiveProbeSummary(path string, live *walletmigration.LiveProbeResult, pass string) {
	fmt.Println("\n=== Live wallet.dat probe ===")
	fmt.Println("path:", path)
	if live.Probe != nil {
		p := live.Probe
		fmt.Printf("is_bdb=%v encrypted=%v keys=%d encrypted_keys=%d pool=%d%s needs_passphrase=%v can_import=%v\n",
			p.IsBDB, p.Encrypted, p.KeyCount, p.EncryptedKeys, p.PoolCount, poolIndexSuffix(p), p.NeedsPassphrase, p.CanImport)
		if p.Hint != "" {
			fmt.Println("hint:", p.Hint)
		}
	}
	if pass != "" {
		if live.ExtractOK {
			fmt.Printf("extract_ok keys=%d\n", live.ExtractedKeys)
		} else if live.ExtractError != "" {
			fmt.Println("extract_error:", live.ExtractError)
		}
	} else if live.Probe != nil && live.Probe.NeedsPassphrase {
		fmt.Println("hint: pass -passphrase or set DOGEGO_WALLET_DAT_PASSPHRASE for decrypt dry-run")
	}
}

func printLiveImportSummary(path string, live *walletmigration.LiveImportResult) {
	fmt.Println("\n=== Live wallet.dat RPC import ===")
	fmt.Println("path:", path)
	if live.Probe != nil {
		p := live.Probe
		fmt.Printf("is_bdb=%v encrypted=%v keys=%d encrypted_keys=%d pool=%d%s needs_passphrase=%v can_import=%v\n",
			p.IsBDB, p.Encrypted, p.KeyCount, p.EncryptedKeys, p.PoolCount, poolIndexSuffix(p), p.NeedsPassphrase, p.CanImport)
	}
	fmt.Printf("status=%s keys_imported=%d\n", live.Status, live.KeysImported)
	if live.KeypoolHint != "" {
		fmt.Println("keypool_hint:", live.KeypoolHint)
	} else if live.Probe != nil && live.Probe.PoolCount > 0 {
		fmt.Println("keypool_hint:", corewallet.PoolKeypoolHint())
	}
	if live.PoolIndicesReplayed != nil {
		fmt.Println("pool_indices_replayed:", *live.PoolIndicesReplayed)
	} else if live.Probe != nil && live.Probe.PoolIndicesReplayed != nil {
		fmt.Println("pool_indices_replayed:", *live.Probe.PoolIndicesReplayed)
	}
	if live.Error != "" {
		fmt.Println("error:", live.Error)
	}
}

func emitWalletMigrationReport(jsonOut bool, report map[string]any) {
	if !jsonOut {
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
}

func poolIndexSuffix(p *corewallet.ProbeResult) string {
	var parts []string
	if p != nil && p.PoolPubkeys > 0 {
		parts = append(parts, fmt.Sprintf("pool_pubkeys=%d", p.PoolPubkeys))
	}
	if p != nil && p.PoolKeysMatched > 0 {
		parts = append(parts, fmt.Sprintf("pool_keys_matched=%d", p.PoolKeysMatched))
	}
	if p != nil && p.PoolKeysUnmatched > 0 {
		parts = append(parts, fmt.Sprintf("pool_keys_unmatched=%d", p.PoolKeysUnmatched))
	}
	if note := corewallet.PoolIndexRangeNote(p); note != "" {
		parts = append(parts, note)
	}
	if p != nil && p.PoolIndicesReplayed != nil {
		parts = append(parts, fmt.Sprintf("pool_indices_replayed=%v", *p.PoolIndicesReplayed))
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}
