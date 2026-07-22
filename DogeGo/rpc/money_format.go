// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "fmt"

// FormatFeeFilterDOGE renders feerate as Core getpeerinfo feefilter (decimal DOGE string).
func FormatFeeFilterDOGE(koinuPerKB uint64) string {
	return fmt.Sprintf("%.8f", float64(koinuPerKB)/1e8)
}
