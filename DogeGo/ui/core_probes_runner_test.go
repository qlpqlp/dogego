// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"

	"dogego/config"
)

func TestRunnerProbeOptionsForConfDefaultSolo(t *testing.T) {
	opts := runnerProbeOptionsForConf("testnet", config.File{})
	if opts.RequireCore {
		t.Fatal("solo testnet should not require Core by default")
	}
}

func TestRunnerProbeOptionsForConfExplicitCore(t *testing.T) {
	opts := runnerProbeOptionsForConf("testnet", config.File{CoreRPCAddr: "127.0.0.1:44555"})
	if !opts.RequireCore {
		t.Fatal("expected RequireCore when core_rpc_addr set")
	}
}

func TestMaintenanceNodeReadyDuringIBD(t *testing.T) {
	if !maintenanceNodeReady(CoreMaintenanceResult{IBD: true, Headers: 100, Blocks: 50}) {
		t.Fatal("expected node ready while catching up")
	}
	if maintenanceNodeReady(CoreMaintenanceResult{IBD: true, Issues: []string{"rpc_unreachable"}}) {
		t.Fatal("expected not ready when RPC unreachable")
	}
}

func TestMaintenanceOperationalOKDuringSync(t *testing.T) {
	m := CoreMaintenanceResult{IBD: true, Blocks: 1000, Headers: 5000, Issues: []string{"blocks_exceed_headers"}}
	if !maintenanceOperationalOK(m) {
		t.Fatal("expected operational OK while catching up blocks to headers")
	}
	if maintenanceOperationalOK(CoreMaintenanceResult{Blocks: 1000, Headers: 500, Issues: []string{"blocks_exceed_headers"}}) {
		t.Fatal("expected fail when not syncing and blocks exceed headers")
	}
}
