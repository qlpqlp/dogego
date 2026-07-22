// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"os"
	"strings"
	"time"

	"dogego/config"
)

// CoreTestResult is returned by POST /api/core-test (quick Core JSON-RPC reachability).
type CoreTestResult struct {
	OK            bool     `json:"ok"`
	CoreAvailable bool     `json:"core_available"`
	CoreRPCAddr   string   `json:"core_rpc_addr"`
	Network       string   `json:"network,omitempty"`
	Chain         string   `json:"chain,omitempty"`
	Blocks        int64    `json:"blocks,omitempty"`
	Headers       int64    `json:"headers,omitempty"`
	Errors        []string `json:"errors,omitempty"`
	Hint          string   `json:"hint,omitempty"`
	TestedAt      string   `json:"tested_at"`
}

// ApplyCoreRPCFormOverride merges optional Settings-form Core RPC fields into probe config.
func ApplyCoreRPCFormOverride(conf config.File, addr, user, pass string) config.File {
	if v := strings.TrimSpace(addr); v != "" {
		conf.CoreRPCAddr = v
	}
	if v := strings.TrimSpace(user); v != "" {
		conf.CoreRPCUser = v
	}
	if pass != "" {
		conf.CoreRPCPassword = pass
	}
	return conf
}

// CoreCompareEnabled reports whether live Core side-by-side checks should run (explicit config or env gate).
func CoreCompareEnabled(network string, conf config.File) bool {
	return CoreRPCExplicitlyConfigured(network, conf) ||
		strings.TrimSpace(os.Getenv("DOGEGO_CORE_COMPARE_REQUIRED")) == "1"
}

// CoreRPCExplicitlyConfigured reports whether Core RPC target was set in config or env (not only defaults).
func CoreRPCExplicitlyConfigured(network string, conf config.File) bool {
	if strings.TrimSpace(conf.CoreRPCAddr) != "" {
		return true
	}
	if strings.TrimSpace(os.Getenv("DOGEGO_CORE_RPC_ADDR")) != "" {
		return true
	}
	if strings.TrimSpace(os.Getenv("DOGEGO_CORE_RPC_PORT")) != "" {
		return true
	}
	return false
}

// ProbeCoreTest calls getblockchaininfo on Dogecoin Core using resolved parity endpoints.
func ProbeCoreTest(network string, conf config.File) CoreTestResult {
	ep := ResolveCoreParityEndpoints(network, conf)
	out := CoreTestResult{
		CoreRPCAddr: ep.Addr,
		Network:     network,
		TestedAt:    time.Now().UTC().Format(time.RFC3339),
		Hint:        "Quick Core JSON-RPC check (getblockchaininfo). Save Settings to persist core_rpc_*.",
	}
	info, err := invokeExternalRPC(ep.Addr, ep.User, ep.Pass, "getblockchaininfo", nil)
	if err != nil {
		out.Errors = append(out.Errors, err.Error())
		out.Hint = "Start Dogecoin Core on loopback or fix core_rpc_addr / RPC auth in Settings → Advanced."
		return out
	}
	out.CoreAvailable = true
	if ch, ok := info["chain"].(string); ok {
		out.Chain = ch
	}
	out.Blocks = jsonInt64(info["blocks"])
	out.Headers = jsonInt64(info["headers"])
	out.OK = true
	return out
}

// AnnotateCoreParitySummary adds lightweight Core side-by-side config hints to /api/summary (no RPC probes).
func AnnotateCoreParitySummary(summary map[string]any, network string, conf config.File) {
	if summary == nil {
		return
	}
	ep := ResolveCoreParityEndpoints(network, conf)
	summary["core_rpc_addr"] = ep.Addr
	summary["core_rpc_configured"] = CoreRPCExplicitlyConfigured(network, conf)
}

func jsonInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	default:
		return 0
	}
}
