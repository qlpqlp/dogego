// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"dogego/applog"
	"dogego/wire"
)

const misbehaviorUnexpectedCompact = 5

// HandleCompactWireUnsupported logs once, then scores repeated compact-block wire on links that did not
// negotiate BIP152 high-bandwidth mode (ephemeral header-sync / block-assist peers reply sendcmpct announce=false).
func HandleCompactWireUnsupported(mw *MsgWriter, addr string, cmd string, mb *MisbehaviorTracker, firstIgnored *bool) {
	if cmd == "" {
		return
	}
	if firstIgnored != nil && !*firstIgnored {
		*firstIgnored = true
		applog.Line("net", "peer "+addr+": ignoring "+cmd+" (no BIP152 HB negotiated on this link)")
		return
	}
	applog.Line("net", "peer "+addr+": unexpected "+cmd+" without BIP152 HB negotiation")
	if mb != nil && addr != "" {
		mb.Note(addr, misbehaviorUnexpectedCompact, "unexpected "+cmd)
	}
	_ = RejectCompactUnsupported(mw, cmd)
}

// RejectCompactUnsupported sends BIP61 reject for unsupported compact-block messages.
func RejectCompactUnsupported(mw *MsgWriter, cmd string) error {
	if mw == nil {
		return nil
	}
	pl, err := wire.EncodeReject("protocol", wire.RejectNonstandard, "compact blocks not supported", nil)
	if err != nil {
		return err
	}
	return mw.Write("reject", pl)
}
