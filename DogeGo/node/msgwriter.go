// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"sync"

	"dogego/wire"
)

// MsgWriter serializes P2P frame writes on one connection. The read loop, block
// fetch, and JSON-RPC relay may all write; TCP requires ordered non-interleaved frames.
type MsgWriter struct {
	mu    sync.Mutex
	conn  net.Conn
	magic [4]byte
	// PeerAddr is the remote host:port (for getpeerinfo last_block / last_transaction).
	PeerAddr string
	// msgStats counts outbound bytes per command (Core getpeerinfo bytessent_per_msg).
	msgStats *peerMsgStats
	// onSent runs after a successful outbound frame (Core getpeerinfo lastsend).
	onSent func()
}

// NewMsgWriter wraps an established P2P connection (post-handshake).
func NewMsgWriter(c net.Conn, magic [4]byte) *MsgWriter {
	return &MsgWriter{conn: c, magic: magic}
}

// Conn returns the underlying connection for ReadMessage and deadlines.
func (w *MsgWriter) Conn() net.Conn { return w.conn }

// Write sends one P2P message (header + payload).
func (w *MsgWriter) Write(cmd string, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	err := wire.WriteMessage(w.conn, w.magic, cmd, payload)
	if err == nil {
		if w.msgStats != nil {
			w.msgStats.addSent(cmd, p2pFrameBytes(len(payload)))
		}
		if w.onSent != nil {
			w.onSent()
		}
	}
	return err
}
