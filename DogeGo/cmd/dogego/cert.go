// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dogego/offlinegate"
)

func runCert(args []string) {
	if len(args) < 1 {
		certUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "offline":
		runCertOffline(args[1:])
	case "autostart":
		runCertAutostart(args[1:])
	case "founder":
		runCertFounder(args[1:])
	case "provision":
		runCertProvision(args[1:])
	case "preflight":
		runCertPreflight(args[1:])
	case "weekly":
		runCertWeekly(args[1:])
	case "weekly-live":
		runCertWeeklyLive(args[1:])
	case "live-soak":
		runCertLiveSoak(args[1:])
	case "bip152-soak", "bip152_soak":
		runCertBip152Soak(args[1:])
	case "enable-weekly":
		runCertEnableWeekly(args[1:])
	case "wallet-migration":
		runCertWalletMigration(args[1:])
	case "wallet-import":
		runCertWalletImport(args[1:])
	case "operator":
		runCertOperator(args[1:])
	case "pq":
		runCertPQ(args[1:])
	case "mining":
		runCertMining(args[1:])
	case "field-evidence":
		runCertFieldEvidence(args[1:])
	case "setup-parity":
		runCertSetupParity(args[1:])
	case "milestones-bde":
		runCertMilestonesBDE(args[1:])
	case "workflow10", "workflow-10":
		runCertWorkflow10(args[1:])
	case "ibd-convergence", "ibd_convergence":
		runCertIBDConvergence(args[1:])
	case "operational":
		runCertOperational(args[1:])
	default:
		certUsage()
		os.Exit(2)
	}
}

func certUsage() {
	base := os.Args[0]
	fmt.Fprintf(os.Stderr, "usage:\n"+
		"  %s cert offline    run offline certification gates (go test; no node or Core required)\n"+
		"  %s cert autostart   verify OS login autostart matches dogecoinconf.json (optional -json -conf PATH)\n"+
		"  %s cert founder     reboot testnet founder preflight vs dogecoinconf.json (optional -json -conf PATH -datadir DIR)\n"+
		"  %s cert provision    dogego-live CI runner provision checklist (optional -json -offline -preflight -run-setup -mine-bootstrap)\n"+
		"  %s cert preflight     dogego-live CI runner preflight (optional -json -offline -require-core -require-wallet-dat)\n"+
		"  %s cert weekly        dogego-live weekly live CI readiness (provision + preflight -require-core; optional -require-wallet-dat -mine-bootstrap)\n"+
		"  %s cert weekly-live   dogego-live scheduled weekly bundle (mirrors ci_scheduled_weekly_live.ps1; optional -mine-bootstrap -require-wallet-dat -include-long-soak -skip-scripts)\n"+
		"  %s cert live-soak       Milestone B multi-hour corruption soak (mirrors ci_milestone_b_full_gate.ps1; optional -duration-min -skip-scripts)\n"+
		"  %s cert bip152-soak     BIP152 AuxPoW/cmpct offline edges + optional live timed soak (-skip-live=false on Windows)\n"+
		"  %s cert enable-weekly set GitHub Actions repo vars for scheduled live CI (requires gh CLI)\n"+
		"  %s cert setup-parity   reboottestnet Core parity setup (preflight + wallet; optional -mine-bootstrap)\n"+
		"  %s cert wallet-migration Core wallet.dat migration cert (go test; optional -offline-only -wallet-dat -passphrase -live-probe -live-import -require-wallet-dat)\n"+
		"  %s cert wallet-import   BIP39/BIP38 + signer + wallet.dat import cert (go test; mirrors wallet_import_cert.ps1)\n"+
		"  %s cert operator        Milestone E standalone operator cert (go test; field-evidence + wallet-import; mirrors operator_workflow_cert.ps1)\n"+
		"  %s cert pq              PQ format/carrier cert (go test; no production PQ safety claim)\n"+
		"  %s cert mining          Mining GBT/aux cert (go test + optional live GET /api/core-mining-probe)\n"+
		"  %s cert field-evidence mainnet field evidence cert (Milestone A offline gates; mirrors field_evidence_cert.ps1)\n"+
		"  %s cert milestones-bde   milestones B/D/E offline close (crash+corpus+operator; live soak still on dogego-live)\n"+
		"  %s cert workflow10      dogego-live workflow 10 orchestrator (optional -enable-github → provision → weekly-live → optional -include-live-soak)\n"+
		"  %s cert ibd-convergence forward IBD progress check (mirrors scripts/ibd_convergence_check.ps1; optional -json -interval-sec N -disk-only)\n"+
		"  %s cert operational   mainnet or reboot testnet operational preflight (-conf PATH -datadir DIR -dual -json)\n"+
		"\n"+
		"Live operator checks (health, Core compare, soak) run inside the node via the web UI\n"+
		"GET /api/core-probes (seventeen live operator-cert gates) and GET /api/core-operator-cert - no PowerShell required.\n"+
		"Optional scripts/ *.ps1 are Windows-oriented CI/operator helpers, not part of the binary.\n"+
		"dogego-live runbook: docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10).\n",
		base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base)
}

func runCertOffline(extra []string) {
	if len(extra) > 0 {
		fmt.Fprintln(os.Stderr, "usage: dogego cert offline")
		os.Exit(2)
	}
	root, err := findGoModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	suites := offlinegate.DefaultSuites()
	fmt.Println("=== DogeGo offline certification (cross-platform) ===")
	fmt.Println("\n> bootstrap consensus/testdata (canonical)")
	if err := offlinegate.RunBootstrap(root, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "\nFAIL: bootstrap consensus/testdata")
		os.Exit(1)
	}
	for _, s := range suites {
		fmt.Printf("\n> go %s\n", strings.Join(s.Args, " "))
		cmd := exec.Command("go", s.Args...)
		cmd.Dir = root
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "\nFAIL: %s\n", s.Name)
			os.Exit(1)
		}
	}
	fmt.Println("\nOffline certification passed.")
}

func findGoModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if st, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !st.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("dogego cert: go.mod not found (run from the DogeGo module directory)")
}
