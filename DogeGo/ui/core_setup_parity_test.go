// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "testing"

func TestProbeSetupParitySkippedOnMainnet(t *testing.T) {
	out := ProbeSetupParity("mainnet")
	if !out.Skipped || !out.OK {
		t.Fatalf("expected skipped OK on mainnet: %+v", out)
	}
}
