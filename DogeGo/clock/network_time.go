// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package clock

// NetworkUnix returns UnixNow adjusted by median peer time offset (Core GetTime analogue).
func NetworkUnix(medianPeerOffsetSec int32) int64 {
	return UnixNow() + int64(medianPeerOffsetSec)
}
