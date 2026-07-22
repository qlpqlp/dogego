// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"

	"dogego/applog"
	"dogego/wire"
)

// ReplySendCmpctDecline answers sendcmpct with announce=false (short-lived header-sync / block-assist links).
func ReplySendCmpctDecline(mw *MsgWriter, payload []byte) (peerWantsCmpctAnnounce bool, err error) {
	peer, err := wire.DecodeSendCmpct(payload)
	if err != nil {
		return false, err
	}
	body, err := wire.DefaultSendCmpctDecline()
	if err != nil {
		return peer.Announce, err
	}
	if err := mw.Write("sendcmpct", body); err != nil {
		return peer.Announce, err
	}
	applog.Line("net", fmt.Sprintf("sendcmpct: peer announce=%v version=%d, replied announce=false (ephemeral link)",
		peer.Announce, peer.Version))
	return peer.Announce, nil
}

// SendCmpctDeclineOnConnect announces full-block preference after verack (Core post-handshake).
func SendCmpctDeclineOnConnect(mw *MsgWriter) error {
	body, err := wire.DefaultSendCmpctDecline()
	if err != nil {
		return err
	}
	return mw.Write("sendcmpct", body)
}
