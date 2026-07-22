// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "dogego/chain"

// IsWitnessEnabled reports whether BIP141 witness rules apply at the next block.
//
// Dogecoin Core hard-disables SegWit in validation.cpp ("Dogecoin: Disable SegWit")
// and sets BIP9 DEPLOYMENT_SEGWIT nTimeout = 0 in chainparams.cpp. DogeGo matches
// that posture: wire decode + weight metrics exist, but consensus/mempool never
// accept witness spends. Do not flip this to true until Core publishes activation
// parameters (see docs/SEGWIT_STATUS.md).
func IsWitnessEnabled(height int64, net chain.Network) bool {
	_ = height
	_ = net
	return false
}
