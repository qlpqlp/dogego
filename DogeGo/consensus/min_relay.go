// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "sync/atomic"

var minRelayTxFeePerKB atomic.Uint64

func init() {
	minRelayTxFeePerKB.Store(uint64(DefaultMinRelayTxFeePerKB))
}

// MinRelayTxFeePerKB returns the configured minimum relay rate (koinu per kB).
func MinRelayTxFeePerKB() uint64 {
	return minRelayTxFeePerKB.Load()
}

// SetMinRelayTxFeePerKB sets the minimum relay rate (Core -minrelaytxfee).
// Values below 1 are ignored (default retained).
func SetMinRelayTxFeePerKB(koinuPerKB uint64) {
	if koinuPerKB < 1 {
		return
	}
	minRelayTxFeePerKB.Store(koinuPerKB)
}
