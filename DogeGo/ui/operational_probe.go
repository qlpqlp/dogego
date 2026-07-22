// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"strings"
	"time"

	"dogego/config"
	"dogego/operational"
)

// OperationalProbeResult is returned by GET /api/core-operational-probe.
type OperationalProbeResult struct {
	CheckedAt string                      `json:"checked_at"`
	OK        bool                        `json:"ok"`
	Network   string                      `json:"network"`
	Dual      bool                        `json:"dual,omitempty"`
	Verify    operational.VerifyResult    `json:"verify,omitempty"`
	DualRun   *operational.DualVerifyResult `json:"dual_run,omitempty"`
	IBD       *CoreIbdConvergenceProbeResult `json:"ibd_convergence,omitempty"`
	CLI       string                      `json:"cli,omitempty"`
	Doc       string                      `json:"doc,omitempty"`
}

// ProbeOperational checks config/disk readiness for this node's network (and optional dual peers).
func ProbeOperational(network, chainDataDir string, conf config.File, dualDataDir string) OperationalProbeResult {
	now := time.Now().UTC().Format(time.RFC3339)
	net := strings.TrimSpace(network)
	if net == "" {
		net = strings.TrimSpace(conf.Network)
	}
	if net == "" {
		net = "mainnet"
	}
	out := OperationalProbeResult{
		CheckedAt: now,
		Network:   net,
		CLI:       "dogego cert operational",
		Doc:       "docs/MAINNET_TESTNET_OPERATIONAL.md",
	}
	ef := conf
	if strings.TrimSpace(ef.DataDir) == "" && strings.TrimSpace(dualDataDir) != "" {
		ef.DataDir = dualDataDir
	}
	vr := operational.Verify(ef)
	out.Verify = vr
	out.OK = vr.OK

	if config.IsRebootTestnetNetwork(net) == false {
		ic := ProbeCoreIbdConvergence(net, chainDataDir, conf.RPCAddr, conf)
		out.IBD = &ic
	}

	dd := strings.TrimSpace(dualDataDir)
	if dd == "" {
		dd = strings.TrimSpace(conf.DataDir)
	}
	if inst, err := config.LoadInstances(dd); err == nil && len(inst.Instances) >= 2 {
		dr := operational.VerifyDual(dd)
		out.Dual = true
		out.DualRun = &dr
		if !dr.OK && vr.OK {
			out.OK = false
		}
	}
	return out
}
