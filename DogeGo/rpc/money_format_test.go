// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestFormatFeeFilterDOGE(t *testing.T) {
	if got := FormatFeeFilterDOGE(100_000); got != "0.00100000" {
		t.Fatalf("got %q", got)
	}
}
