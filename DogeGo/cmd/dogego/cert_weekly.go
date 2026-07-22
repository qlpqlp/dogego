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

func runCertWeekly(args []string) {
	fs := flag.NewFlagSet("cert weekly", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON report")
	skipPreflight := fs.Bool("skip-preflight", false, "provision checklist only (no live RPC preflight)")
	requireWalletDat := fs.Bool("require-wallet-dat", false, "fail unless DOGEGO_WALLET_DAT probes/imports successfully")
	mineBootstrap := fs.Bool("mine-bootstrap", false, "run setup-parity before preflight (reboottestnet wallet bootstrap)")
	dogePort := fs.Int("dogego-port", 44556, "DogeGo reboottestnet RPC port for -mine-bootstrap")
	corePort := fs.Int("core-port", 44555, "Core reboottestnet RPC port for -mine-bootstrap")
	_ = fs.Parse(args)

	var setupParity *runner.SetupParityResult
	if *mineBootstrap && !*skipPreflight {
		sp := runner.VerifySetupParity(runner.SetupParityOptions{
			MineBootstrap: true,
			DogeGoPort:    *dogePort,
			CorePort:      *corePort,
		})
		setupParity = &sp
		runner.ApplySetupParityEnv(sp.EnvExports)
		if !sp.OK {
			if *jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(map[string]any{"ok": false, "setup_parity": sp})
			} else {
				fmt.Println("=== DogeGo weekly live CI readiness (dogego-live) ===")
				fmt.Println("\nSetup parity failed:")
				for _, i := range sp.Issues {
					fmt.Println("  FAIL:", i)
				}
				fmt.Println("\nSee docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10)")
			}
			os.Exit(1)
		}
	}

	prov := runner.VerifyProvision(runner.ProvisionOptions{Preflight: !*skipPreflight})
	var pf runner.PreflightResult
	if !*skipPreflight {
		importWallet := runner.WalletDatImportEnabled(*requireWalletDat)
		pf = runner.RunPreflight(runner.PreflightOptions{
			RequireCore:      true,
			RequireWalletDat: *requireWalletDat,
			WalletDatImport:  importWallet,
		})
	}
	ok := prov.OK && (*skipPreflight || pf.OK)

	if *jsonOut {
		out := map[string]any{
			"ok":        ok,
			"provision": prov,
		}
		if setupParity != nil {
			out["setup_parity"] = setupParity
		}
		if !*skipPreflight {
			out["preflight"] = pf
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	} else {
		fmt.Println("=== DogeGo weekly live CI readiness (dogego-live) ===")
		if prov.Doc != "" {
			fmt.Println("DOC:", prov.Doc)
		}
		if setupParity != nil {
			fmt.Println("\nSetup parity passed (mine-bootstrap).")
		}
		fmt.Printf("\nProvision checklist (%d/%d)\n", prov.Done, prov.Total)
		for _, row := range prov.Checklist {
			mark := "[ ]"
			if row.Done {
				mark = "[x]"
			}
			fmt.Printf("  %s %d. %s\n", mark, row.Step, row.Item)
		}
		if !*skipPreflight {
			fmt.Println("\nRPC preflight (require Core)")
			for _, n := range pf.Notes {
				fmt.Println("  NOTE:", n)
			}
			for _, w := range pf.Warnings {
				fmt.Println("  WARN:", w)
			}
			for _, i := range pf.Issues {
				fmt.Println("  FAIL:", i)
			}
			if pf.WalletMigration != nil && pf.WalletMigration.Probe != nil {
				p := pf.WalletMigration.Probe
				fmt.Printf("  wallet.dat: keys=%d encrypted_keys=%d pool=%d%s needs_passphrase=%v can_import=%v\n",
					p.KeyCount, p.EncryptedKeys, p.PoolCount, poolIndexSuffix(p), p.NeedsPassphrase, p.CanImport)
			}
			if pf.WalletDatImport != nil {
				fmt.Printf("  wallet.dat import: status=%s keys_imported=%d\n",
					pf.WalletDatImport.Status, pf.WalletDatImport.KeysImported)
				if pf.WalletDatImport.Error != "" {
					fmt.Println("  wallet.dat import error:", pf.WalletDatImport.Error)
				}
			}
		}
		if ok {
			fmt.Println("\nWeekly live readiness passed.")
			fmt.Println("Next: run dogego cert enable-weekly (or scripts/gh_enable_scheduled_live.ps1)")
		} else {
			fmt.Println("\nWeekly live readiness incomplete.")
			fmt.Println("Fix checklist items, then: dogego cert provision -preflight && dogego cert preflight -require-core")
			fmt.Println("See docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10)")
		}
	}
	if !ok {
		os.Exit(1)
	}
}
