// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"sync"
)

type dgrTunnelPushRegistry struct {
	mu    sync.Mutex
	chans map[string]chan []byte
}

var tunnelPush dgrTunnelPushRegistry

func registerTunnelPushPeer(peer string) chan []byte {
	key := normalizeTunnelPeerKey(peer)
	ch := make(chan []byte, 32)
	tunnelPush.mu.Lock()
	if tunnelPush.chans == nil {
		tunnelPush.chans = make(map[string]chan []byte)
	}
	tunnelPush.chans[key] = ch
	tunnelPush.mu.Unlock()
	return ch
}

func unregisterTunnelPushPeer(peer string) {
	key := normalizeTunnelPeerKey(peer)
	tunnelPush.mu.Lock()
	delete(tunnelPush.chans, key)
	tunnelPush.mu.Unlock()
}

// DeliverTunnelPush delivers an unsolicited P2P wire frame from the operator tunnel pool.
func DeliverTunnelPush(peer string, wireMsg []byte) {
	if len(wireMsg) == 0 {
		return
	}
	key := normalizeTunnelPeerKey(peer)
	tunnelPush.mu.Lock()
	ch := tunnelPush.chans[key]
	tunnelPush.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- append([]byte(nil), wireMsg...):
	default:
	}
}

func normalizeTunnelPeerKey(peer string) string {
	host, port, err := net.SplitHostPort(peer)
	if err != nil {
		return peer
	}
	if ip := net.ParseIP(host); ip != nil {
		return net.JoinHostPort(ip.String(), port)
	}
	return peer
}
