// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package node

import (
	"fmt"
	"net"

	"dogego/applog"
	"dogego/extensions"
	"dogego/wire"
)

func maybeNegotiateExtensions(conn net.Conn, magic [4]byte, peerAddr string, mgr *extensions.Manager, mw *MsgWriter) {
	if mgr == nil || conn == nil || peerAddr == "" {
		return
	}
	enabled, err := mgr.Negotiate(conn, magic, peerAddr)
	if err != nil {
		applog.Line("extensions", fmt.Sprintf("peer %s negotiation: %v", peerAddr, err))
		return
	}
	if len(enabled) > 0 {
		applog.Line("extensions", fmt.Sprintf("peer %s overlays: %v", peerAddr, enabled))
		mgr.NotifyPeerNegotiated(peerAddr, enabled, extensionSendFunc(mw, magic))
	}
}

func handleExtensionP2P(mgr *extensions.Manager, peerAddr, cmd string, payload []byte, send func(string, []byte) error) bool {
	if mgr == nil {
		return false
	}
	handled, err := mgr.HandleP2PMessage(peerAddr, cmd, payload, send)
	if err != nil {
		applog.Line("extensions", fmt.Sprintf("peer %s %s: %v", peerAddr, cmd, err))
	}
	return handled
}

func extensionSendFunc(mw *MsgWriter, magic [4]byte) func(string, []byte) error {
	if mw == nil {
		return nil
	}
	return func(cmd string, payload []byte) error {
		return wire.WriteMessage(mw.Conn(), magic, cmd, payload)
	}
}
