// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"dogego/wallet/corewallet"
)

// ChecklistItem is one dogego-live runner provisioning step.
type ChecklistItem struct {
	Step int    `json:"step"`
	Item string `json:"item"`
	Done bool   `json:"done"`
}

// ProvisionOptions configures VerifyProvision.
type ProvisionOptions struct {
	Preflight     bool
	OfflineOnly   bool
	RunSetup      bool
	MineBootstrap bool
	MineBlocks    int
	DogeGoPort    int
	CorePort      int
	ProbeHost     string
	ProbeTimeout  time.Duration
	RPCTimeout    time.Duration
}

// VerifyResult reports dogego-live CI runner readiness (mirrors ci_runner_provision_checklist.ps1).
type VerifyResult struct {
	OK          bool            `json:"ok"`
	Done        int             `json:"done"`
	Total       int             `json:"total"`
	Checklist   []ChecklistItem `json:"checklist"`
	Auto        []string        `json:"auto,omitempty"`
	Issues      []string        `json:"issues,omitempty"`
	Warnings    []string        `json:"warnings,omitempty"`
	Notes       []string        `json:"notes,omitempty"`
	Doc         string          `json:"doc,omitempty"`
	Preflight   bool                    `json:"preflight,omitempty"`
	OfflineOnly bool                    `json:"offline_only,omitempty"`
	SetupParity *SetupParityResult      `json:"setup_parity,omitempty"`
}

func defaultProvisionOptions() ProvisionOptions {
	return ProvisionOptions{
		DogeGoPort:   44556,
		CorePort:     44555,
		ProbeHost:    "127.0.0.1",
		ProbeTimeout: 500 * time.Millisecond,
	}
}

// VerifyProvision evaluates runner provisioning checklist items detectable without PowerShell.
func VerifyProvision(opts ProvisionOptions) VerifyResult {
	def := defaultProvisionOptions()
	if opts.DogeGoPort == 0 {
		opts.DogeGoPort = def.DogeGoPort
	}
	if opts.CorePort == 0 {
		opts.CorePort = def.CorePort
	}
	if opts.ProbeHost == "" {
		opts.ProbeHost = def.ProbeHost
	}
	if opts.ProbeTimeout <= 0 {
		opts.ProbeTimeout = def.ProbeTimeout
	}

	checklist := []ChecklistItem{
		{Step: 1, Item: "Install Go 1.22+ and add to PATH"},
		{Step: 2, Item: "Install dogecoin-cli (Core) and set DOGEGO_CORE_CLI if non-default"},
		{Step: 3, Item: "Register GitHub self-hosted runner with label dogego-live"},
		{Step: 4, Item: "Run DogeGo node on reboottestnet RPC :44556 with wallet enabled"},
		{Step: 5, Item: "Run Core on reboottestnet RPC :44555 with wallet enabled"},
		{Step: 6, Item: "Set DOGEGO_SCHEDULED_WEEKLY_LIVE=1 and DOGEGO_SCHEDULED_LIVE_SOAK=1 (or run gh_enable_scheduled_live.ps1)"},
		{Step: 7, Item: "Run dogego cert setup-parity -mine-bootstrap (or setup_reboottestnet_core_parity.ps1 -MineBootstrap)"},
		{Step: 8, Item: "Dispatch DogeGo workflow with live_core_gate or live_e2e"},
		{Step: 9, Item: "Optional: provision Core wallet.dat (scripts/provision_wallet_dat_fixture.ps1 -SetUserEnv)"},
	}

	r := VerifyResult{
		Total:       len(checklist),
		Checklist:   checklist,
		Doc:         DogegoLiveWorkflow10Doc,
		Preflight:   opts.Preflight,
		OfflineOnly: opts.OfflineOnly,
	}

	if hasGo() {
		checklist[0].Done = true
		r.Auto = append(r.Auto, "go_ok")
		if out, err := exec.Command("go", "version").CombinedOutput(); err == nil {
			r.Notes = append(r.Notes, strings.TrimSpace(string(out)))
		}
	} else {
		r.Issues = append(r.Issues, "go_missing")
	}

	if hasCoreCLI() {
		checklist[1].Done = true
		r.Auto = append(r.Auto, "core_cli_ok")
	} else {
		r.Issues = append(r.Issues, "core_cli_missing")
	}

	if hasRunnerLabel() {
		checklist[2].Done = true
		r.Auto = append(r.Auto, "runner_label_ok")
	} else {
		r.Issues = append(r.Issues, "runner_label_missing")
	}

	if strings.TrimSpace(os.Getenv("DOGEGO_SCHEDULED_WEEKLY_LIVE")) == "1" ||
		strings.TrimSpace(os.Getenv("DOGEGO_SCHEDULED_CORE_GATE")) == "1" {
		checklist[5].Done = true
		r.Auto = append(r.Auto, "weekly_live_env_ok")
	}
	if strings.TrimSpace(os.Getenv("DOGEGO_SCHEDULED_LIVE_SOAK")) == "1" {
		r.Auto = append(r.Auto, "live_soak_env_ok")
		if checklist[5].Done {
			r.Notes = append(r.Notes, "scheduled_live_soak=1")
		}
	}

	if setupParityEnvConfigured() {
		checklist[6].Done = true
		r.Auto = append(r.Auto, "setup_parity_env_ok")
		r.Notes = append(r.Notes, "hint: dogego cert setup-parity -mine-bootstrap sets DOGEGO_CORE_COMPARE* for 24/24 gate")
	}

	if p := strings.TrimSpace(os.Getenv("DOGEGO_WALLET_DAT")); p != "" {
		if noteWalletDatProvision(&r, p) {
			checklist[8].Done = true
			r.Auto = append(r.Auto, "wallet_dat_fixture_ok")
		}
	} else if discovered := ResolveWalletDatPath(""); discovered != "" {
		if noteWalletDatProvision(&r, discovered) {
			r.Notes = append(r.Notes, "hint: set DOGEGO_WALLET_DAT via scripts/provision_wallet_dat_fixture.ps1 -SetUserEnv")
		}
	}
	if strings.TrimSpace(os.Getenv("DOGEGO_WALLET_DAT_REQUIRED")) == "1" {
		r.Notes = append(r.Notes, "DOGEGO_WALLET_DAT_REQUIRED=1")
	}

	if opts.Preflight && !opts.OfflineOnly {
		if portOpen(opts.ProbeHost, opts.DogeGoPort, opts.ProbeTimeout) {
			checklist[3].Done = true
			r.Auto = append(r.Auto, "dogego_rpc_listen")
		} else {
			r.Issues = append(r.Issues, "dogego_rpc_unreachable")
		}
		if portOpen(opts.ProbeHost, opts.CorePort, opts.ProbeTimeout) {
			checklist[4].Done = true
			r.Auto = append(r.Auto, "core_rpc_listen")
		} else {
			r.Issues = append(r.Issues, "core_rpc_unreachable")
		}
	}

	if opts.RunSetup {
		if opts.OfflineOnly {
			r.Issues = append(r.Issues, "run_setup_requires_live")
		} else {
			rpcTimeout := opts.RPCTimeout
			if rpcTimeout <= 0 {
				rpcTimeout = 15 * time.Second
			}
			sp := VerifySetupParity(SetupParityOptions{
				MineBootstrap: opts.MineBootstrap,
				MineBlocks:    opts.MineBlocks,
				DogeGoPort:    opts.DogeGoPort,
				CorePort:      opts.CorePort,
				Host:          opts.ProbeHost,
				RPCTimeout:    rpcTimeout,
			})
			r.SetupParity = &sp
			ApplySetupParityEnv(sp.EnvExports)
			r.Notes = append(r.Notes, sp.Notes...)
			r.Warnings = append(r.Warnings, sp.Warnings...)
			if sp.OK {
				checklist[6].Done = true
				r.Auto = append(r.Auto, "setup_parity_ok")
			} else {
				r.Issues = append(r.Issues, "setup_parity_failed")
				r.Issues = append(r.Issues, sp.Issues...)
			}
		}
	}

	done := 0
	for i := range checklist {
		r.Checklist[i] = checklist[i]
		if checklist[i].Done {
			done++
		}
	}
	r.Done = done
	r.OK = done >= 4
	if opts.RunSetup {
		switch {
		case opts.OfflineOnly:
			r.OK = false
		case r.SetupParity != nil:
			r.OK = r.SetupParity.OK
		default:
			r.OK = false
		}
	}
	return r
}

func noteWalletDatProvision(r *VerifyResult, path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	probe, err := corewallet.ProbeWalletDat(path, 0x9e)
	if err != nil || probe == nil || !probe.IsBDB {
		return false
	}
	r.Notes = append(r.Notes, "wallet_dat="+path)
	r.Notes = append(r.Notes, walletDatProbeNote(probe))
	if probe.PoolCount > 0 {
		r.Notes = append(r.Notes, "wallet_dat_keypool_hint="+corewallet.PoolKeypoolHint())
	}
	return true
}

func setupParityEnvConfigured() bool {
	return strings.TrimSpace(os.Getenv("DOGEGO_CORE_COMPARE")) == "1" &&
		strings.TrimSpace(os.Getenv("DOGEGO_CORE_COMPARE_REQUIRED")) == "1" &&
		strings.TrimSpace(os.Getenv("DOGEGO_CORE_COMPARE_MIN")) == "24"
}

func hasGo() bool {
	_, err := exec.LookPath("go")
	return err == nil
}

func hasCoreCLI() bool {
	if p := strings.TrimSpace(os.Getenv("DOGEGO_CORE_CLI")); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return true
		}
	}
	if _, err := exec.LookPath("dogecoin-cli"); err == nil {
		return true
	}
	return false
}

func hasRunnerLabel() bool {
	if strings.TrimSpace(os.Getenv("RUNNER_NAME")) != "" {
		return true
	}
	labels := strings.ToLower(os.Getenv("GITHUB_RUNNER_LABELS"))
	return strings.Contains(labels, "dogego-live")
}

func portOpen(host string, port int, timeout time.Duration) bool {
	addr := net.JoinHostPort(host, itoa(port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
