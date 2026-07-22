// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"dogego/wire"
)

const (
	p2pProxyDialTimeout  = 15 * time.Second
	p2pProxyReadTimeout  = 45 * time.Second
	p2pProxyWriteTimeout = 30 * time.Second
	p2pProxyIdleRead     = 3 * time.Second // one-way frames (sendheaders, …)
)

// proxyP2PFrame dials peer TCP, sends one wire frame, reads one response frame.
func proxyP2PFrame(peer string, wireMsg []byte, magic [4]byte) (resp []byte, status byte) {
	if len(wireMsg) == 0 || len(wireMsg) > maxP2PFrameWire {
		return nil, P2PProxyWireErr
	}
	dialer := net.Dialer{Timeout: p2pProxyDialTimeout}
	conn, err := dialer.Dial("tcp", peer)
	if err != nil {
		return nil, P2PProxyDialFail
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(p2pProxyWriteTimeout))
	if _, err = conn.Write(wireMsg); err != nil {
		return nil, P2PProxyWireErr
	}
	_ = conn.SetDeadline(time.Now().Add(p2pProxyIdleRead))
	cmd, payload, err := wire.ReadMessage(conn, magic)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return nil, P2PProxyNoResponse
		}
		return nil, P2PProxyWireErr
	}
	var buf bytes.Buffer
	if err := wire.WriteMessage(&buf, magic, cmd, payload); err != nil {
		return nil, P2PProxyWireErr
	}
	return buf.Bytes(), P2PProxyOK
}

func p2pProxyStatusText(status byte) string {
	switch status {
	case P2PProxyOK:
		return "ok"
	case P2PProxyDialFail:
		return "dial failed"
	case P2PProxyTimeout:
		return "timeout"
	case P2PProxyWireErr:
		return "wire error"
	case P2PProxyNoResponse:
		return "no immediate response"
	default:
		return fmt.Sprintf("status %d", status)
	}
}
