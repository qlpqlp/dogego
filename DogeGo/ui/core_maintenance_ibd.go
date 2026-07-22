// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

// maintenanceSyncing reports initial block download or blocks still catching up to headers.
func maintenanceSyncing(m CoreMaintenanceResult) bool {
	if m.IBD {
		return true
	}
	return m.Headers > 0 && m.Blocks < m.Headers
}

// maintenanceOperationalOK is true when maintenance RPCs are healthy, or only sync-transient
// checks remain during IBD (solo operator view - cert strictness uses len(Issues) separately).
func maintenanceOperationalOK(m CoreMaintenanceResult) bool {
	if len(m.Issues) == 0 {
		return true
	}
	if !maintenanceSyncing(m) {
		return false
	}
	for _, iss := range m.Issues {
		switch iss {
		case "rpc_unreachable", "dogego_rpc_not_ready", "getblockchaininfo_failed", "getindexinfo_failed", "verifychain_failed":
			return false
		}
	}
	return true
}
