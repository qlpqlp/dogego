// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestDialP2POutboundPrefersDGRWhenCGNAT(t *testing.T) {
	t.Parallel()
	ConfigureDGRTunnelDial(
		func(peer string, wireMsg []byte, timeout time.Duration) ([]byte, error) {
			return nil, nil
		},
		func() bool { return true },
		true,
		[4]byte{0xfa, 0x0f, 0xa6, 0xfd},
	)
	defer ClearDGRTunnelDial()

	c, tunneled, err := DialP2POutbound(context.Background(), net.Dialer{Timeout: 50 * time.Millisecond}, "127.0.0.1:44556")
	if err != nil {
		t.Fatal(err)
	}
	if !tunneled {
		t.Fatal("expected tunneled dial")
	}
	if _, ok := c.(*dgrTunnelConn); !ok {
		t.Fatal("expected dgrTunnelConn")
	}
	_ = c.Close()
}

func TestDialP2POutboundTCPFirstWhenNotCGNAT(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		_ = c.Close()
	}()

	ConfigureDGRTunnelDial(
		func(peer string, wireMsg []byte, timeout time.Duration) ([]byte, error) {
			return nil, errors.New("should not use dgr")
		},
		func() bool { return true },
		false,
		[4]byte{0xfa, 0x0f, 0xa6, 0xfd},
	)
	defer ClearDGRTunnelDial()

	_, tunneled, err := DialP2POutbound(context.Background(), net.Dialer{Timeout: 2 * time.Second}, ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if tunneled {
		t.Fatal("expected direct TCP")
	}
}
