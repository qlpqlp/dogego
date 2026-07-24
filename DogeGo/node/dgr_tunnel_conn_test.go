// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"dogego/chain"
	"dogego/wire"
)

func TestDGRTunnelConnPingPong(t *testing.T) {
	t.Parallel()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	var magic = p.Magic
	relay := func(peer string, wireMsg []byte, timeout time.Duration) ([]byte, error) {
		cmd, payload, err := wire.ReadMessage(bytes.NewReader(wireMsg), magic)
		if err != nil {
			return nil, err
		}
		if cmd != "ping" {
			t.Fatalf("cmd=%q", cmd)
		}
		var buf bytes.Buffer
		if err := wire.WriteMessage(&buf, magic, "pong", payload); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	conn, err := NewDGRTunnelConn("127.0.0.1:44556", magic, relay)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, ok := conn.RemoteAddr().(*net.TCPAddr); !ok {
		t.Fatal("expected TCPAddr remote")
	}
	payload := []byte{1, 2, 3, 4}
	if err := wire.WriteMessage(conn, magic, "ping", payload); err != nil {
		t.Fatal(err)
	}
	cmd, back, err := wire.ReadMessage(conn, magic)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "pong" || !bytes.Equal(back, payload) {
		t.Fatalf("pong mismatch cmd=%q payload=%x", cmd, back)
	}
}

func TestDGRTunnelConnHandshake(t *testing.T) {
	t.Parallel()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	var magic = p.Magic
	var calls int
	relay := func(peer string, wireMsg []byte, timeout time.Duration) ([]byte, error) {
		calls++
		cmd, _, err := wire.ReadMessage(bytes.NewReader(wireMsg), magic)
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		switch calls {
		case 1:
			if cmd != "version" {
				t.Fatalf("call1 cmd=%q", cmd)
			}
			ver := wire.BuildVersionPayload(p.ProtocolVersion, p.NodeNetwork, net.IPv4(127, 0, 0, 1), 44556, 99, "/mock/", 100, true)
			if err := wire.WriteMessage(&buf, magic, "version", ver); err != nil {
				return nil, err
			}
		case 2:
			if cmd != "verack" {
				t.Fatalf("call2 cmd=%q", cmd)
			}
			if err := wire.WriteMessage(&buf, magic, "verack", nil); err != nil {
				return nil, err
			}
		default:
			return nil, nil
		}
		return buf.Bytes(), nil
	}
	conn, err := NewDGRTunnelConn("127.0.0.1:44556", magic, relay)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	dv, err := Handshake(context.Background(), conn, p, "/dogego-test/", p.NodeNetwork)
	if err != nil {
		t.Fatal(err)
	}
	if dv == nil || dv.StartHeight != 100 {
		t.Fatalf("handshake dv=%+v", dv)
	}
}
