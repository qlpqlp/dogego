// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import "testing"

func TestHistoryDeferReason(t *testing.T) {
	if got := HistoryDeferReason(true, 0, true, true, true, 200); got != "ibd_active" {
		t.Fatalf("ibd=%q", got)
	}
	if got := HistoryDeferReason(false, 128, true, true, true, 200); got != "connect_lag" {
		t.Fatalf("lag=%q", got)
	}
	if got := HistoryDeferReason(false, 0, true, true, true, 200); got != "scan_building" {
		t.Fatalf("scan=%q", got)
	}
	if got := HistoryDeferReason(false, 0, true, true, true, 8); got != "" {
		t.Fatalf("few utxos=%q", got)
	}
}
