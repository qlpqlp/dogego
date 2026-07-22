// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import (
	"bytes"
	"net"
	"sync"
	"time"

	"dogego/wire"
)

const (
	tunnelPoolIdleClose = 3 * time.Minute
	tunnelProxyIdleRead = 3 * time.Second
)

type tunnelKey struct {
	session uint64
	peer    string
}

type pooledConn struct {
	conn     net.Conn
	lastUsed time.Time
	reading  bool
}

type p2pTunnelPool struct {
	mu    sync.Mutex
	magic [4]byte
	conns map[tunnelKey]*pooledConn
	push  func(sessionID uint64, peer string, wireMsg []byte)
}

func newP2PTunnelPool(magic [4]byte, push func(uint64, string, []byte)) *p2pTunnelPool {
	return &p2pTunnelPool{
		magic: magic,
		conns: make(map[tunnelKey]*pooledConn),
		push:  push,
	}
}

func (p *p2pTunnelPool) Proxy(sessionID uint64, peer string, wireMsg []byte) ([]byte, byte) {
	if p == nil {
		return proxyP2PFrame(peer, wireMsg, [4]byte{})
	}
	if len(wireMsg) == 0 || len(wireMsg) > maxP2PFrameWire {
		return nil, P2PProxyWireErr
	}
	key := tunnelKey{session: sessionID, peer: peer}
	pc, err := p.getOrDial(key)
	if err != nil {
		return nil, P2PProxyDialFail
	}
	_ = pc.conn.SetDeadline(time.Now().Add(p2pProxyWriteTimeout))
	if _, err = pc.conn.Write(wireMsg); err != nil {
		p.drop(key)
		return nil, P2PProxyWireErr
	}
	p.mu.Lock()
	pc.lastUsed = time.Now()
	p.mu.Unlock()

	_ = pc.conn.SetDeadline(time.Now().Add(tunnelProxyIdleRead))
	cmd, payload, err := wire.ReadMessage(pc.conn, p.magic)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return nil, P2PProxyNoResponse
		}
		p.drop(key)
		return nil, P2PProxyWireErr
	}
	var buf bytes.Buffer
	if err := wire.WriteMessage(&buf, p.magic, cmd, payload); err != nil {
		return nil, P2PProxyWireErr
	}
	return buf.Bytes(), P2PProxyOK
}

func (p *p2pTunnelPool) getOrDial(key tunnelKey) (*pooledConn, error) {
	p.mu.Lock()
	if pc, ok := p.conns[key]; ok && pc != nil && pc.conn != nil {
		pc.lastUsed = time.Now()
		p.mu.Unlock()
		return pc, nil
	}
	p.mu.Unlock()

	dialer := net.Dialer{Timeout: p2pProxyDialTimeout}
	conn, err := dialer.Dial("tcp", key.peer)
	if err != nil {
		return nil, err
	}
	pc := &pooledConn{conn: conn, lastUsed: time.Now()}
	p.mu.Lock()
	if old, ok := p.conns[key]; ok && old != nil && old.conn != nil {
		p.mu.Unlock()
		_ = conn.Close()
		return old, nil
	}
	p.conns[key] = pc
	startRead := !pc.reading
	pc.reading = true
	p.mu.Unlock()
	if startRead {
		go p.readLoop(key, pc)
	}
	return pc, nil
}

func (p *p2pTunnelPool) readLoop(key tunnelKey, pc *pooledConn) {
	for {
		p.mu.Lock()
		cur, ok := p.conns[key]
		if !ok || cur != pc || pc.conn == nil {
			p.mu.Unlock()
			return
		}
		conn := pc.conn
		p.mu.Unlock()

		_ = conn.SetDeadline(time.Now().Add(tunnelPoolIdleClose))
		cmd, payload, err := wire.ReadMessage(conn, p.magic)
		if err != nil {
			p.drop(key)
			return
		}
		var buf bytes.Buffer
		if err := wire.WriteMessage(&buf, p.magic, cmd, payload); err != nil {
			p.drop(key)
			return
		}
		if p.push != nil {
			p.push(key.session, key.peer, append([]byte(nil), buf.Bytes()...))
		}
		p.mu.Lock()
		if cur, ok := p.conns[key]; ok && cur == pc {
			pc.lastUsed = time.Now()
		}
		p.mu.Unlock()
	}
}

func (p *p2pTunnelPool) dropSession(sessionID uint64) {
	p.mu.Lock()
	var keys []tunnelKey
	for k := range p.conns {
		if k.session == sessionID {
			keys = append(keys, k)
		}
	}
	p.mu.Unlock()
	for _, k := range keys {
		p.drop(k)
	}
}

func (p *p2pTunnelPool) drop(key tunnelKey) {
	p.mu.Lock()
	pc, ok := p.conns[key]
	if ok {
		delete(p.conns, key)
	}
	p.mu.Unlock()
	if ok && pc != nil && pc.conn != nil {
		_ = pc.conn.Close()
	}
}
