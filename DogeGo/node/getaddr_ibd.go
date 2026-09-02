// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"time"

	"dogego/applog"
)

// ibdGetAddrInterval is how often we ask connected peers for more addresses during block catch-up.
// Core gossips continuously; keep this short so assist dials find NODE_NETWORK archival peers.
const ibdGetAddrInterval = 45 * time.Second

// RequestGetAddrFromPeers sends an empty getaddr to the primary link and all relay outbounds.
func RequestGetAddrFromPeers(mw *MsgWriter, pm *PeerMgr) {
	if mw != nil {
		if err := mw.Write("getaddr", nil); err != nil {
			applog.Line("net", "getaddr (primary): "+err.Error())
		} else if pm != nil && mw.PeerAddr != "" {
			pm.NoteOutboundGetAddr(mw.PeerAddr)
		}
	}
	if pm != nil {
		pm.BroadcastGetAddr()
	}
}

// MaybeRequestGetAddrDuringIBD polls peers for addresses while block bodies lag headers.
func MaybeRequestGetAddrDuringIBD(mw *MsgWriter, pm *PeerMgr, ibdActive bool, last *time.Time) {
	if !ibdActive || last == nil {
		return
	}
	if time.Since(*last) < ibdGetAddrInterval {
		return
	}
	*last = time.Now()
	applog.Line("net", "requesting peer addresses (getaddr) during block catch-up")
	RequestGetAddrFromPeers(mw, pm)
}
