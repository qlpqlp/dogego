// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"dogego/clock"
	"dogego/wire"
)

// wireNetworkUnix returns Core GetTime for header/block validation (median peer offset when available).
func wireNetworkUnix(peerMgr *PeerMgr, handshakePeer *wire.DecodedVersion) int64 {
	if peerMgr != nil {
		return clock.NetworkUnix(peerMgr.MedianTimeOffset())
	}
	if handshakePeer != nil {
		return clock.NetworkUnix(wire.TimeOffsetSeconds(handshakePeer, clock.UnixNow()))
	}
	return clock.UnixNow()
}
