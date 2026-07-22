// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// ApplyNodeRelayFees applies Core init.cpp -incrementalrelayfee / -minrelaytxfee policy.
// When minRelayExplicit is false and incremental exceeds the default min relay, min relay is raised to match incremental.
func ApplyNodeRelayFees(configIncremental, configMinRelay uint64, minRelayExplicit bool) {
	if configIncremental > 0 {
		SetIncrementalRelayFeePerKB(configIncremental)
	}
	incr := IncrementalRelayFeePerKB()
	if minRelayExplicit {
		if configMinRelay > 0 {
			SetMinRelayTxFeePerKB(configMinRelay)
		}
		return
	}
	if incr > MinRelayTxFeePerKB() {
		SetMinRelayTxFeePerKB(incr)
	}
}
