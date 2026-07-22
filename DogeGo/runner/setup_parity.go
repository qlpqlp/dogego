// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// SetupParityOptions configures VerifySetupParity (mirrors setup_reboottestnet_core_parity.ps1).
type SetupParityOptions struct {
	MineBootstrap bool
	MineBlocks    int
	DogeGoPort    int
	CorePort      int
	Host          string
	RPCTimeout    time.Duration
}

// SetupParityResult reports reboottestnet DogeGo + Core wallet readiness for stateful mempool gates.
type SetupParityResult struct {
	OK            bool            `json:"ok"`
	Preflight     PreflightResult `json:"preflight"`
	DogeGoBalance float64         `json:"dogego_balance,omitempty"`
	CoreBalance   *float64        `json:"core_balance,omitempty"`
	MinedBlocks   int             `json:"mined_blocks,omitempty"`
	Notes         []string        `json:"notes,omitempty"`
	Warnings      []string        `json:"warnings,omitempty"`
	Issues        []string        `json:"issues,omitempty"`
	EnvExports    []string        `json:"env_exports,omitempty"`
	NextSteps     []string        `json:"next_steps,omitempty"`
	Doc           string          `json:"doc,omitempty"`
}

// VerifySetupParity prepares reboottestnet for Core-aligned stateful mempool gates (24/24).
func VerifySetupParity(opts SetupParityOptions) SetupParityResult {
	if opts.DogeGoPort == 0 {
		opts.DogeGoPort = 44556
	}
	if opts.CorePort == 0 {
		opts.CorePort = 44555
	}
	if opts.Host == "" {
		opts.Host = "127.0.0.1"
	}
	if opts.RPCTimeout <= 0 {
		opts.RPCTimeout = 15 * time.Second
	}
	if opts.MineBlocks <= 0 {
		opts.MineBlocks = 101
	}

	r := SetupParityResult{
		Doc: DogegoLiveWorkflow10Doc,
		EnvExports: []string{
			"DOGEGO_CORE_COMPARE=1",
			"DOGEGO_CORE_COMPARE_REQUIRED=1",
			"DOGEGO_CORE_COMPARE_MIN=24",
			"DOGEGO_CORE_RPC_PORT=" + strconv.Itoa(opts.CorePort),
		},
		NextSteps: []string{
			"scripts/core_reboottestnet_core_aligned_gate.ps1",
			"dogego cert enable-weekly (sets DOGEGO_SCHEDULED_WEEKLY_LIVE + DOGEGO_SCHEDULED_CORE_GATE + DOGEGO_SCHEDULED_LIVE_SOAK)",
		},
	}

	pf := RunPreflight(PreflightOptions{
		RequireCore: true,
		DogeGoPort:  opts.DogeGoPort,
		CorePort:    opts.CorePort,
		Host:        opts.Host,
		RPCTimeout:  opts.RPCTimeout,
	})
	r.Preflight = pf
	if !pf.OK {
		r.Issues = append(r.Issues, "preflight_failed")
		r.Issues = append(r.Issues, pf.Issues...)
		return r
	}

	user := strings.TrimSpace(os.Getenv("DOGEGO_RPC_USER"))
	pass := os.Getenv("DOGEGO_RPC_PASS")
	wallet, err := invokeJSONRPC(opts.Host, opts.DogeGoPort, user, pass, "getwalletinfo", nil, opts.RPCTimeout)
	if err != nil {
		r.Issues = append(r.Issues, "dogego_wallet_unreachable")
		r.Notes = append(r.Notes, err.Error())
		return r
	}
	r.DogeGoBalance = walletBalance(wallet)
	r.Notes = append(r.Notes, fmt.Sprintf("dogego_balance=%g", r.DogeGoBalance))

	if opts.MineBootstrap && r.DogeGoBalance < 1.0 {
		addr, err := invokeJSONRPCString(opts.Host, opts.DogeGoPort, user, pass, "getnewaddress", []any{"core_parity_bootstrap"}, opts.RPCTimeout)
		if err != nil {
			r.Issues = append(r.Issues, "getnewaddress_failed")
			r.Notes = append(r.Notes, err.Error())
			return r
		}
		if addr == "" {
			r.Issues = append(r.Issues, "getnewaddress_empty")
			return r
		}
		hashes, err := invokeJSONRPCStringSlice(opts.Host, opts.DogeGoPort, user, pass, "generatetoaddress", []any{opts.MineBlocks, addr}, opts.RPCTimeout)
		if err != nil {
			r.Issues = append(r.Issues, "generatetoaddress_failed")
			r.Notes = append(r.Notes, err.Error())
			return r
		}
		if len(hashes) == 0 {
			r.Issues = append(r.Issues, "generatetoaddress_empty")
			return r
		}
		r.MinedBlocks = opts.MineBlocks
		wallet, err = invokeJSONRPC(opts.Host, opts.DogeGoPort, user, pass, "getwalletinfo", nil, opts.RPCTimeout)
		if err == nil {
			r.DogeGoBalance = walletBalance(wallet)
			r.Notes = append(r.Notes, fmt.Sprintf("dogego_balance_after_mine=%g", r.DogeGoBalance))
		}
	}

	if cli := resolveCoreCLI(); cli != "" {
		coreUser := strings.TrimSpace(os.Getenv("DOGEGO_CORE_RPC_USER"))
		corePass := os.Getenv("DOGEGO_CORE_RPC_PASS")
		coreWallet, err := invokeCoreCLI(cli, opts.CorePort, coreUser, corePass, "getwalletinfo")
		if err != nil {
			r.Warnings = append(r.Warnings, "core_wallet_unavailable")
			r.Notes = append(r.Notes, "stateful Core compare may fail on wallet-anchored rows without Core wallet")
		} else {
			bal := walletBalance(coreWallet)
			r.CoreBalance = &bal
			r.Notes = append(r.Notes, fmt.Sprintf("core_balance=%g", bal))
		}
	}

	r.OK = len(r.Issues) == 0
	return r
}

// ApplySetupParityEnv sets DOGEGO_CORE_COMPARE* exports in the current process (KEY=value strings).
func ApplySetupParityEnv(exports []string) {
	for _, e := range exports {
		key, val, ok := strings.Cut(strings.TrimSpace(e), "=")
		if !ok || key == "" {
			continue
		}
		_ = os.Setenv(key, val)
	}
}

func walletBalance(m map[string]any) float64 {
	if m == nil {
		return 0
	}
	switch v := m["balance"].(type) {
	case float64:
		return v
	case json.Number:
		f, _ := v.Float64()
		return f
	default:
		return 0
	}
}
