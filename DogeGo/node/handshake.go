// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"dogego/chain"
	"dogego/wire"
)

// Handshake completes version/verack with a peer using chain params.
// On success returns a decoded copy of the peer's first "version" message (for getpeerinfo / logs).
// Handshake completes version/verack. localServices is NODE_* bits (0 → p.NodeNetwork only).
func Handshake(ctx context.Context, conn net.Conn, p chain.Params, userAgent string, localServices uint64) (*wire.DecodedVersion, error) {
	tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		return nil, fmt.Errorf("expected TCP remote address")
	}
	var nonceBuf [8]byte
	if _, err := rand.Read(nonceBuf[:]); err != nil {
		return nil, err
	}
	nonce := binary.LittleEndian.Uint64(nonceBuf[:])
	if localServices == 0 {
		localServices = p.NodeNetwork
	}
	payload := wire.BuildVersionPayload(p.ProtocolVersion, localServices, tcpAddr.IP, uint16(tcpAddr.Port), nonce, userAgent, 0, true)
	if err := wire.WriteMessage(conn, p.Magic, "version", payload); err != nil {
		return nil, err
	}
	gotVer := false
	var peer *wire.DecodedVersion
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(20 * time.Second))
		cmd, pl, err := wire.ReadMessage(conn, p.Magic)
		if err != nil {
			return nil, err
		}
		switch cmd {
		case "version":
			if !gotVer {
				gotVer = true
				dv, err := wire.ParseVersionPayload(pl)
				if err != nil {
					return nil, fmt.Errorf("peer version: %w", err)
				}
				peer = dv
				if err := wire.WriteMessage(conn, p.Magic, "verack", nil); err != nil {
					return nil, err
				}
			}
		case "verack":
			if !gotVer {
				return nil, fmt.Errorf("verack before version")
			}
			if err := wire.WriteMessage(conn, p.Magic, "sendheaders", nil); err != nil {
				return nil, err
			}
			if body, err := wire.DefaultSendCmpctDecline(); err == nil {
				_ = wire.WriteMessage(conn, p.Magic, "sendcmpct", body)
			}
			return peer, nil
		case "reject":
			rj, err := wire.DecodeRejectPayload(pl)
			if err != nil {
				return nil, fmt.Errorf("peer reject (unparseable): %w", err)
			}
			return nil, fmt.Errorf("peer sent reject: %s", rj.String())
		default:
			// ignore sendheaders, etc.
		}
	}
}
