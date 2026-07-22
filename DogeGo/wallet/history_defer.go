// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

const (
	// HistoryConnectLagDefer matches web UI / GET /api/wallet/txs defer when connect lag exceeds this.
	HistoryConnectLagDefer int64 = 64
	// HistoryScanBuildUtxoMin matches defer during rescan when spendable UTXO count exceeds this.
	HistoryScanBuildUtxoMin = 64
)

// HistoryDeferReason returns ibd_active, connect_lag, scan_building, or empty when history may load.
func HistoryDeferReason(ibd bool, connectLag int64, scanning, utxoWalk, scanPending bool, utxoCount int) string {
	if ibd {
		return "ibd_active"
	}
	if connectLag > HistoryConnectLagDefer {
		return "connect_lag"
	}
	if !scanning {
		return ""
	}
	if !utxoWalk && !scanPending {
		return ""
	}
	if utxoCount > HistoryScanBuildUtxoMin {
		return "scan_building"
	}
	return ""
}
