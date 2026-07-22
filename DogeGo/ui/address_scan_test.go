// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "testing"

func TestScanAddressInRawWindowNoJournal(t *testing.T) {
	_, err := ScanAddressInRawWindow(nil, nil, nil, 0x71, 0x42, "x", nil, "", -1, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
