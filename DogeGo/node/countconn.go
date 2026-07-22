// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"sync/atomic"
)

// netByteCounter tracks raw TCP bytes for the single outbound P2P link (used by getnettotals).
type netByteCounter struct {
	recv atomic.Uint64
	sent atomic.Uint64
}

func newNetByteCounter() *netByteCounter { return &netByteCounter{} }

func (n *netByteCounter) Recv() uint64 { return n.recv.Load() }
func (n *netByteCounter) Sent() uint64 { return n.sent.Load() }

func (n *netByteCounter) addRecv(m int) {
	if m > 0 {
		n.recv.Add(uint64(m))
	}
}

func (n *netByteCounter) addSent(m int) {
	if m > 0 {
		n.sent.Add(uint64(m))
	}
}

// countingConn wraps a net.Conn to count Read/Write bytes (P2P wire framing + payloads).
type countingConn struct {
	net.Conn
	ctr *netByteCounter
}

func (c *countingConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.ctr.addRecv(n)
	return n, err
}

func (c *countingConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	c.ctr.addSent(n)
	return n, err
}
