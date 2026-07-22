// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import (
	"bytes"
	"net"
	"testing"

	"dogego/wire"
)

func TestP2PFrameRequestRoundTrip(t *testing.T) {
	t.Parallel()
	wireMsg := []byte{0xfa, 0x0f, 0xa6, 0xfd, 'p', 'i', 'n', 'g'}
	payload, err := encodeP2PFrameRequest(42, "127.0.0.1:44556", wireMsg)
	if err != nil {
		t.Fatal(err)
	}
	id, peer, back, ok := decodeP2PFrameRequest(payload)
	if !ok || id != 42 || peer != "127.0.0.1:44556" || !bytes.Equal(back, wireMsg) {
		t.Fatalf("decode mismatch id=%d peer=%q ok=%v", id, peer, ok)
	}
}

func TestP2PFrameResponseRoundTrip(t *testing.T) {
	t.Parallel()
	respWire := []byte{1, 2, 3, 4, 5}
	payload, err := encodeP2PFrameResponse(7, P2PProxyOK, respWire)
	if err != nil {
		t.Fatal(err)
	}
	id, st, back, ok := decodeP2PFrameResponse(payload)
	if !ok || id != 7 || st != P2PProxyOK || !bytes.Equal(back, respWire) {
		t.Fatalf("decode mismatch id=%d st=%d ok=%v", id, st, ok)
	}
	errPayload, err := encodeP2PFrameResponse(8, P2PProxyDialFail, nil)
	if err != nil {
		t.Fatal(err)
	}
	id, st, back, ok = decodeP2PFrameResponse(errPayload)
	if !ok || id != 8 || st != P2PProxyDialFail || len(back) != 0 {
		t.Fatalf("err response mismatch id=%d st=%d len=%d", id, st, len(back))
	}
}

func TestProxyP2PFramePingPong(t *testing.T) {
	var magic [4]byte
	copy(magic[:], []byte{0xfa, 0x0f, 0xa6, 0xfd})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		cmd, payload, err := wire.ReadMessage(conn, magic)
		if err != nil || cmd != "ping" {
			return
		}
		_ = wire.WriteMessage(conn, magic, "pong", payload)
	}()
	var req bytes.Buffer
	if err := wire.WriteMessage(&req, magic, "ping", []byte{0x01, 0x02, 0x03, 0x04}); err != nil {
		t.Fatal(err)
	}
	resp, status := proxyP2PFrame(ln.Addr().String(), req.Bytes(), magic)
	if status != P2PProxyOK {
		t.Fatalf("status=%d want ok", status)
	}
	cmd, payload, err := wire.ReadMessage(bytes.NewReader(resp), magic)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "pong" || len(payload) != 4 {
		t.Fatalf("cmd=%q payload=%x", cmd, payload)
	}
}
