// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

const dgrTunnelProxyTimeout = 50 * time.Second

// DGRTunnelRelay proxies one full P2P wire frame to a remote peer via DGR.
type DGRTunnelRelay func(peer string, wireMsg []byte, timeout time.Duration) ([]byte, error)

// dgrTunnelConn implements net.Conn by sending each complete P2P frame through DGR P2P_FRAME.
type dgrTunnelConn struct {
	remote net.Addr
	magic  [4]byte
	relay  DGRTunnelRelay
	pushCh chan []byte

	mu           sync.Mutex
	readBuf      []byte
	readOff      int
	writeBuf     bytes.Buffer
	readDeadline time.Time
	writeDeadline time.Time
	closed       bool
}

// NewDGRTunnelConn dials a P2P peer through an active outbound DGR relay session.
func NewDGRTunnelConn(peer string, magic [4]byte, relay DGRTunnelRelay) (*dgrTunnelConn, error) {
	if relay == nil {
		return nil, fmt.Errorf("dgr tunnel: relay not configured")
	}
	host, portStr, err := net.SplitHostPort(peer)
	if err != nil {
		return nil, fmt.Errorf("dgr tunnel: invalid peer %q: %w", peer, err)
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil {
		return nil, fmt.Errorf("dgr tunnel: invalid port in %q: %w", peer, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("dgr tunnel: cannot resolve host %q", host)
		}
		ip = ips[0]
	}
	remote := &net.TCPAddr{IP: ip, Port: port}
	return &dgrTunnelConn{
		remote: remote,
		magic:  magic,
		relay:  relay,
		pushCh: registerTunnelPushPeer(remote.String()),
	}, nil
}

func (c *dgrTunnelConn) Read(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, net.ErrClosed
	}
	if err := c.checkDeadline(c.readDeadline); err != nil {
		return 0, err
	}
	if c.readOff >= len(c.readBuf) {
		if err := c.waitPushLocked(); err != nil {
			return 0, err
		}
	}
	n := copy(b, c.readBuf[c.readOff:])
	c.readOff += n
	if c.readOff >= len(c.readBuf) {
		c.readBuf = nil
		c.readOff = 0
	}
	return n, nil
}

func (c *dgrTunnelConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, net.ErrClosed
	}
	if err := c.checkDeadline(c.writeDeadline); err != nil {
		return 0, err
	}
	n, err := c.writeBuf.Write(p)
	if err != nil {
		return n, err
	}
	if err := c.flushFramesLocked(); err != nil {
		return n, err
	}
	return n, nil
}

func (c *dgrTunnelConn) flushFramesLocked() error {
	for c.writeBuf.Len() >= 24 {
		hdr := c.writeBuf.Bytes()[:24]
		if !bytes.Equal(hdr[0:4], c.magic[:]) {
			return fmt.Errorf("dgr tunnel: bad magic in outbound frame")
		}
		n := binary.LittleEndian.Uint32(hdr[16:20])
		if n > 32*1024*1024 {
			return fmt.Errorf("dgr tunnel: oversized frame %d", n)
		}
		total := int(24 + n)
		if c.writeBuf.Len() < total {
			return nil
		}
		frame := append([]byte(nil), c.writeBuf.Bytes()[:total]...)
		c.writeBuf.Next(total)
		remote := c.remote.String()
		c.mu.Unlock()
		resp, err := c.relay(remote, frame, dgrTunnelProxyTimeout)
		c.mu.Lock()
		if err != nil {
			return err
		}
		if len(resp) > 0 {
			c.readBuf = append(c.readBuf[c.readOff:], resp...)
			c.readOff = 0
		}
	}
	return nil
}

func (c *dgrTunnelConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		unregisterTunnelPushPeer(c.remote.String())
	}
	c.closed = true
	c.readBuf = nil
	c.writeBuf.Reset()
	return nil
}

func (c *dgrTunnelConn) waitPushLocked() error {
	if c.closed {
		return net.ErrClosed
	}
	if err := c.checkDeadline(c.readDeadline); err != nil {
		return err
	}
	if c.pushCh == nil {
		return c.timeoutErr(c.readDeadline)
	}
	if !c.readDeadline.IsZero() {
		timeout := time.Until(c.readDeadline)
		if timeout <= 0 {
			return c.timeoutErr(c.readDeadline)
		}
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case frame := <-c.pushCh:
			c.readBuf = frame
			c.readOff = 0
			return nil
		case <-timer.C:
			return c.timeoutErr(c.readDeadline)
		}
	}
	select {
	case frame := <-c.pushCh:
		c.readBuf = frame
		c.readOff = 0
		return nil
	default:
		return errors.New("dgr tunnel: read would block")
	}
}

func (c *dgrTunnelConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4zero, Port: 0}
}

func (c *dgrTunnelConn) RemoteAddr() net.Addr { return c.remote }

func (c *dgrTunnelConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readDeadline = t
	c.writeDeadline = t
	return nil
}

func (c *dgrTunnelConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readDeadline = t
	return nil
}

func (c *dgrTunnelConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writeDeadline = t
	return nil
}

func (c *dgrTunnelConn) checkDeadline(deadline time.Time) error {
	if deadline.IsZero() {
		return nil
	}
	if time.Now().After(deadline) {
		return c.timeoutErr(deadline)
	}
	return nil
}

func (c *dgrTunnelConn) timeoutErr(deadline time.Time) error {
	if deadline.IsZero() {
		return errors.New("dgr tunnel: read would block")
	}
	return &net.OpError{Op: "read", Net: "dgr", Addr: c.remote, Err: fmt.Errorf("i/o timeout")}
}
