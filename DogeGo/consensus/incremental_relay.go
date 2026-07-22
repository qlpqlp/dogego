// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "sync/atomic"

var incrementalRelayFeePerKB atomic.Uint64

func init() {
	incrementalRelayFeePerKB.Store(uint64(DefaultIncrementalRelayFeePerKB))
}

// IncrementalRelayFeePerKB returns the configured incremental relay rate (koinu per kB).
func IncrementalRelayFeePerKB() uint64 {
	return incrementalRelayFeePerKB.Load()
}

// SetIncrementalRelayFeePerKB sets the incremental relay rate (Core -incrementalrelayfee).
// Values below 1 are ignored (default retained).
func SetIncrementalRelayFeePerKB(koinuPerKB uint64) {
	if koinuPerKB < 1 {
		return
	}
	incrementalRelayFeePerKB.Store(koinuPerKB)
}
