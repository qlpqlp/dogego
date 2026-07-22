// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"net"
	"strings"
	"sync"

	"dogego/wire"
)

// P2PProtocol is an extension overlay protocol (e.g. zkproof-v1).
type P2PProtocol interface {
	ProtocolID() string
	// P2PCommands returns overlay command names this protocol handles.
	P2PCommands() []string
	HandleP2P(cmd string, payload []byte, peer string, send func(cmd string, payload []byte) error) error
}

// P2PExtension adds peer overlay support to an Extension.
type P2PExtension interface {
	Extension
	P2PProtocol
}

// PeerExtState tracks negotiated protocols for one peer.
type PeerExtState struct {
	Enabled []string
}

// peerExtTable maps peer address to negotiated overlay protocols.
type peerExtTable struct {
	mu    sync.RWMutex
	peers map[string]PeerExtState
}

func newPeerExtTable() *peerExtTable {
	return &peerExtTable{peers: make(map[string]PeerExtState)}
}

func (t *peerExtTable) set(peer string, st PeerExtState) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.peers[peer] = st
	t.mu.Unlock()
}

func (t *peerExtTable) enabled(peer, proto string) bool {
	if t == nil {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	st, ok := t.peers[peer]
	if !ok {
		return false
	}
	for _, p := range st.Enabled {
		if p == proto {
			return true
		}
	}
	return false
}

// Negotiate runs exthello/extack after Dogecoin verack. Unknown peers continue as normal Dogecoin peers.
func (m *Manager) Negotiate(conn net.Conn, magic [4]byte, peerAddr string) ([]string, error) {
	if m == nil || conn == nil {
		return nil, nil
	}
	local := m.localProtocolIDs()
	if len(local) == 0 {
		return nil, nil
	}
	hello := EncodeExtHello(local)
	if err := wire.WriteMessage(conn, magic, CmdExtHello, hello); err != nil {
		return nil, err
	}
	cmd, pl, err := wire.ReadMessage(conn, magic)
	if err != nil {
		// Peer may be Core-only; not an error.
		return nil, nil
	}
	switch cmd {
	case CmdExtHello:
		peerSup, err := DecodeExtHello(pl)
		if err != nil {
			return nil, nil
		}
		enabled := intersectProtocols(local, peerSup)
		ack := EncodeExtAck(enabled)
		_ = wire.WriteMessage(conn, magic, CmdExtAck, ack)
		m.notePeerProtocols(peerAddr, enabled)
		return enabled, nil
	case CmdExtAck:
		enabled, err := DecodeExtAck(pl)
		if err == nil {
			m.notePeerProtocols(peerAddr, enabled)
			return enabled, nil
		}
	}
	return nil, nil
}

func (m *Manager) localProtocolIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.localProtocolIDsLocked()
}

func (m *Manager) localProtocolIDsLocked() []string {
	var out []string
	for _, ext := range m.active {
		if pe, ok := ext.(P2PExtension); ok {
			out = append(out, pe.ProtocolID())
		}
	}
	return out
}

func intersectProtocols(a, b []string) []string {
	set := make(map[string]struct{}, len(a))
	for _, s := range a {
		set[strings.TrimSpace(s)] = struct{}{}
	}
	var out []string
	for _, s := range b {
		s = strings.TrimSpace(s)
		if _, ok := set[s]; ok {
			out = append(out, s)
		}
	}
	return out
}

func (m *Manager) notePeerProtocols(peer string, enabled []string) {
	if m.peerExts == nil {
		m.peerExts = newPeerExtTable()
	}
	m.peerExts.set(peer, PeerExtState{Enabled: append([]string(nil), enabled...)})
}

// HandleP2PMessage dispatches overlay commands from enabled extensions.
func (m *Manager) HandleP2PMessage(peerAddr, cmd string, payload []byte, send func(string, []byte) error) (handled bool, err error) {
	if m == nil {
		return false, nil
	}
	switch cmd {
	case CmdExtHello:
		peerSup, decErr := DecodeExtHello(payload)
		if decErr != nil {
			return true, nil
		}
		m.mu.Lock()
		enabled := intersectProtocols(m.localProtocolIDsLocked(), peerSup)
		m.notePeerProtocols(peerAddr, enabled)
		m.mu.Unlock()
		if send != nil {
			_ = send(CmdExtAck, EncodeExtAck(enabled))
			m.NotifyPeerNegotiated(peerAddr, enabled, send)
		}
		return true, nil
	case CmdExtAck:
		enabled, decErr := DecodeExtAck(payload)
		if decErr == nil {
			m.notePeerProtocols(peerAddr, enabled)
		}
		return true, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, ext := range m.active {
		man := m.activeManifest[id]
		if !man.HasPermission("p2p_extension") {
			continue
		}
		pe, ok := ext.(P2PExtension)
		if !ok {
			continue
		}
		if m.peerExts != nil && !m.peerExts.enabled(peerAddr, pe.ProtocolID()) {
			continue
		}
		for _, c := range pe.P2PCommands() {
			if c == cmd {
				if err := pe.HandleP2P(cmd, payload, peerAddr, send); err != nil {
					return true, err
				}
				return true, nil
			}
		}
	}
	return false, nil
}

// UnregisterPeer clears overlay send hooks and negotiated protocols when a P2P session ends.
func (m *Manager) UnregisterPeer(peerAddr string) {
	if m == nil || peerAddr == "" {
		return
	}
	m.UnregisterPeerOverlay(peerAddr)
	if m.peerExts != nil {
		m.peerExts.mu.Lock()
		delete(m.peerExts.peers, peerAddr)
		m.peerExts.mu.Unlock()
	}
}

// PeerEnabledProtocols returns negotiated overlay ids for a peer.
func (m *Manager) PeerEnabledProtocols(peerAddr string) []string {
	if m == nil || m.peerExts == nil {
		return nil
	}
	m.peerExts.mu.RLock()
	defer m.peerExts.mu.RUnlock()
	st, ok := m.peerExts.peers[peerAddr]
	if !ok {
		return nil
	}
	return append([]string(nil), st.Enabled...)
}
