// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

// NoteCmpctWireIgnored handles cmpctblock/blocktxn/getblocktxn on links without negotiated BIP152 HB (see cmpct_peer.go).
func (l *peerLink) NoteCmpctWireIgnored(mw *MsgWriter, cmd string, mb *MisbehaviorTracker) {
	if l == nil || cmd == "" {
		return
	}
	HandleCompactWireUnsupported(mw, l.addr, cmd, mb, &l.cmpctWireIgnored)
}
