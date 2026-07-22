// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"fmt"
	"strings"
	"time"

	"dogego/config"
	"dogego/ibdconvergence"
	"dogego/walletmigration"
)

// CoreIbdConvergenceProbeResult is returned by GET /api/core-ibd-convergence-probe.
type CoreIbdConvergenceProbeResult struct {
	CheckedAt       string                          `json:"checked_at"`
	OK              bool                            `json:"ok"`
	Skipped         bool                            `json:"skipped,omitempty"`
	Reason          string                          `json:"reason,omitempty"`
	Snapshot        ibdconvergence.ProgressSnapshot `json:"snapshot"`
	BodyCoveragePct float64                         `json:"body_coverage_pct,omitempty"`
	ConnectBoost    string                          `json:"connect_boost,omitempty"`
	Issues          []string                        `json:"issues,omitempty"`
	Notes           []string                        `json:"notes,omitempty"`
	Hint            string                          `json:"hint,omitempty"`
}

func ibdProbeRPCClient(rpcAddr string, conf config.File) walletmigration.RPCClient {
	c := walletmigration.DefaultRPCClient()
	addr := strings.TrimSpace(rpcAddr)
	if addr != "" {
		if !strings.Contains(addr, "://") {
			addr = "http://" + addr
		}
		c.BaseURL = strings.TrimRight(addr, "/")
	}
	if u := strings.TrimSpace(conf.RpcUser); u != "" {
		c.User = u
	}
	if p := strings.TrimSpace(conf.RpcPassword); p != "" {
		c.Pass = p
	}
	return c
}

// ProbeCoreIbdConvergence collects a single IBD progress snapshot (RPC + web + disk).
// Timed convergence windows remain in dogego cert ibd-convergence / scripts/ibd_convergence_check.ps1.
func ProbeCoreIbdConvergence(network, chainDataDir, rpcAddr string, conf config.File) CoreIbdConvergenceProbeResult {
	_ = network
	out := CoreIbdConvergenceProbeResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Hint:      "Snapshot only. Timed window: dogego cert ibd-convergence -interval-sec 120 or scripts/ibd_convergence_check.ps1",
	}
	opts := ibdconvergence.SnapshotOptions{
		ChainDir:   strings.TrimSpace(chainDataDir),
		RPCTimeout: 45 * time.Second,
	}
	if strings.TrimSpace(chainDataDir) == "" && strings.TrimSpace(rpcAddr) == "" {
		opts.DiskOnly = true
	} else if strings.TrimSpace(rpcAddr) != "" || strings.TrimSpace(conf.RpcUser) != "" {
		opts.RPC = ibdProbeRPCClient(rpcAddr, conf)
	}
	snap, err := ibdconvergence.CollectSnapshot(opts)
	if err != nil {
		out.Issues = append(out.Issues, err.Error())
		if strings.TrimSpace(chainDataDir) == "" && strings.TrimSpace(rpcAddr) == "" {
			out.Skipped = true
			out.Reason = "rpc_not_ready"
			out.OK = true
			out.Notes = append(out.Notes, "configure RPC or chain datadir for snapshot")
			return out
		}
		out.Reason = "snapshot_failed"
		return out
	}
	out.Snapshot = snap
	out.ConnectBoost = snap.ConnectBoost
	if snap.Headers != nil && snap.Contiguous != nil && *snap.Headers > 0 {
		out.BodyCoveragePct = float64(*snap.Contiguous) / float64(*snap.Headers) * 100
	}
	out.Notes = append(out.Notes, snap.FormatLine())
	out.OK = true
	if snap.IBD != nil && *snap.IBD {
		out.Notes = append(out.Notes, "forward_ibd_active")
	} else if snap.Contiguous != nil && snap.Headers != nil && *snap.Contiguous >= *snap.Headers {
		out.Notes = append(out.Notes, "bodies_caught_up")
	} else if out.BodyCoveragePct > 0 {
		out.Notes = append(out.Notes, fmt.Sprintf("body_coverage=%.1f%%", out.BodyCoveragePct))
	}
	return out
}
