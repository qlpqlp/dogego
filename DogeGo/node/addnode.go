// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"net"
	"strings"
	"sync"
)

// AddedNodeStore tracks manually added peers (Core addnode add/remove; onetry is not stored).
type AddedNodeStore struct {
	mu    sync.Mutex
	nodes map[string]struct{}
}

// NewAddedNodeStore creates an empty added-node list.
func NewAddedNodeStore() *AddedNodeStore {
	return &AddedNodeStore{nodes: make(map[string]struct{})}
}

// NormalizeNodeAddr returns host:port for addnode RPC (default chain port when port omitted).
func NormalizeNodeAddr(node string, defaultPort int) (string, error) {
	node = strings.TrimSpace(node)
	if node == "" {
		return "", fmt.Errorf("node address is invalid")
	}
	if strings.Contains(node, ":") {
		host, port, err := net.SplitHostPort(node)
		if err != nil {
			return "", fmt.Errorf("node address is invalid")
		}
		if host == "" || port == "" {
			return "", fmt.Errorf("node address is invalid")
		}
		return net.JoinHostPort(host, port), nil
	}
	if defaultPort <= 0 || defaultPort > 65535 {
		return "", fmt.Errorf("node address is invalid")
	}
	return net.JoinHostPort(node, fmt.Sprintf("%d", defaultPort)), nil
}

// relaySeedHostP2PAddrs maps relay QUIC seeds (host:24433) to Dogecoin P2P addnode targets (host:p2pPort).
func relaySeedHostP2PAddrs(seeds []string, p2pPort int) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, seed := range seeds {
		seed = strings.TrimSpace(seed)
		if seed == "" {
			continue
		}
		host := seed
		if strings.Contains(host, ":") {
			h, _, err := net.SplitHostPort(host)
			if err == nil && h != "" {
				host = h
			}
		}
		addr, err := NormalizeNodeAddr(host, p2pPort)
		if err != nil {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		out = append(out, addr)
	}
	return out
}

// Add records a persistent peer target.
func (s *AddedNodeStore) Add(addr string) {
	if s == nil || addr == "" {
		return
	}
	s.mu.Lock()
	s.nodes[addr] = struct{}{}
	s.mu.Unlock()
}

// Remove drops a persistent peer target.
func (s *AddedNodeStore) Remove(addr string) {
	if s == nil || addr == "" {
		return
	}
	s.mu.Lock()
	delete(s.nodes, addr)
	s.mu.Unlock()
}

// Contains reports whether addr matches a persistent addnode entry (host:port normalized).
func (s *AddedNodeStore) Contains(addr string) bool {
	if s == nil || addr == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for a := range s.nodes {
		if addnodeMatchesSession(a, addr) {
			return true
		}
	}
	return false
}

// List returns added node host:ports in stable order.
func (s *AddedNodeStore) List() []string {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.nodes))
	for a := range s.nodes {
		out = append(out, a)
	}
	return out
}

// SeedPeerMgr adds all persistent nodes to the peer manager address pool.
func (s *AddedNodeStore) SeedPeerMgr(pm *PeerMgr) {
	if s == nil || pm == nil {
		return
	}
	for _, a := range s.List() {
		pm.NoteAddr(a)
		pm.noteAddnodePersistent(a)
	}
}
