// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds DGR counters and live session rows (safe for concurrent snapshot).
type Metrics struct {
	mu sync.RWMutex

	Enabled           bool
	InboundRole       bool
	OutboundRole      bool
	ListenAddr        string
	ListenBound       string
	AdvertiseAddr     string
	RelayPort         int
	ListenerOK        bool
	Health            string
	HealthMessage     string
	ServiceBitHex     string
	UsingRelay        bool
	ActiveRelayAddr   string
	ActiveRelayCert   string
	ServerCertSHA256  string
	RegisteredClients int
	OutboundSessions  int
	InboundSessions   int

	RegisterOK   atomic.Uint64
	RegisterFail atomic.Uint64
	DialAttempts atomic.Uint64
	DialOK       atomic.Uint64
	DialFail     atomic.Uint64
	FramesIn     atomic.Uint64
	FramesOut    atomic.Uint64
	Pings        atomic.Uint64
	Pongs        atomic.Uint64
	InvTxIn      atomic.Uint64
	InvTxOut     atomic.Uint64
	P2PFramesIn  atomic.Uint64
	P2PFramesOut atomic.Uint64
	P2PProxyOK   atomic.Uint64
	P2PProxyFail atomic.Uint64
	P2PPublishIn atomic.Uint64
	P2PPublishOut atomic.Uint64
	P2PPushIn    atomic.Uint64
	P2PPushOut   atomic.Uint64
	P2PTunnelIn  atomic.Uint64
	P2PTunnelOut atomic.Uint64
	PeerHintsIn  atomic.Uint64
	PeerHintsOut atomic.Uint64
	TLSPinOK     atomic.Uint64
	TLSPinFail   atomic.Uint64
	RateLimited  atomic.Uint64
	BytesIn      atomic.Uint64
	BytesOut     atomic.Uint64

	startedAt time.Time
	clients   []clientRow
	outbound  []sessionRow
	discovery []string
}

type clientRow struct {
	Remote     string    `json:"remote"`
	Network    string    `json:"network,omitempty"`
	P2PPort    int       `json:"p2p_port,omitempty"`
	Since      time.Time `json:"since"`
	LastActive time.Time `json:"last_active"`
	FramesIn   uint64    `json:"frames_in"`
	FramesOut  uint64    `json:"frames_out"`
}

type sessionRow struct {
	Addr       string    `json:"addr"`
	Since      time.Time `json:"since"`
	LastActive time.Time `json:"last_active"`
	Registered bool      `json:"registered"`
	FramesIn   uint64    `json:"frames_in"`
	FramesOut  uint64    `json:"frames_out"`
}

func newMetrics() *Metrics {
	return &Metrics{startedAt: time.Now()}
}

func (m *Metrics) noteClient(row clientRow) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.clients {
		if m.clients[i].Remote == row.Remote {
			m.clients[i] = row
			m.RegisteredClients = len(m.clients)
			return
		}
	}
	m.clients = append(m.clients, row)
	m.RegisteredClients = len(m.clients)
}

func (m *Metrics) dropClient(remote string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.clients[:0]
	for _, c := range m.clients {
		if c.Remote != remote {
			out = append(out, c)
		}
	}
	m.clients = out
	m.RegisteredClients = len(m.clients)
}

func (m *Metrics) noteOutbound(row sessionRow) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.outbound {
		if m.outbound[i].Addr == row.Addr {
			m.outbound[i] = row
			m.OutboundSessions = len(m.outbound)
			m.syncUsingRelayLocked()
			return
		}
	}
	m.outbound = append(m.outbound, row)
	m.OutboundSessions = len(m.outbound)
	m.syncUsingRelayLocked()
}

func (m *Metrics) dropOutbound(addr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.outbound[:0]
	for _, s := range m.outbound {
		if s.Addr != addr {
			out = append(out, s)
		}
	}
	m.outbound = out
	m.OutboundSessions = len(m.outbound)
	m.syncUsingRelayLocked()
}

func (m *Metrics) setDiscovery(addrs []string) {
	m.mu.Lock()
	m.discovery = append([]string(nil), addrs...)
	m.mu.Unlock()
}

// SetAdvertiseHost sets the public host:port CGNAT clients should dial (inbound rdogego).
func (m *Metrics) SetAdvertiseHost(host string, port int) {
	if m == nil || port <= 0 {
		return
	}
	host = trimAdvertiseHost(host)
	if host == "" {
		return
	}
	m.mu.Lock()
	m.AdvertiseAddr = host + ":" + strconv.Itoa(port)
	m.mu.Unlock()
}

func trimAdvertiseHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if i := strings.LastIndex(host, ":"); i > 0 && strings.Contains(host, ".") {
		return host[:i]
	}
	if strings.HasPrefix(host, "[") {
		if j := strings.Index(host, "]"); j > 0 {
			return host[:j+1]
		}
	}
	return host
}

func (m *Metrics) syncUsingRelayLocked() {
	m.UsingRelay = false
	m.ActiveRelayAddr = ""
	for _, s := range m.outbound {
		if s.Registered {
			m.UsingRelay = true
			m.ActiveRelayAddr = s.Addr
			break
		}
	}
}

func (m *Metrics) computeHealthLocked() (string, string) {
	if !m.Enabled {
		return "off", "DGR disabled"
	}
	if m.InboundRole {
		if m.ListenerOK {
			msg := "QUIC relay listening"
			if m.ListenBound != "" {
				msg = "QUIC relay listening on " + m.ListenBound
			}
			if m.AdvertiseAddr != "" {
				msg += " (advertise " + m.AdvertiseAddr + ")"
			}
			return "ok", msg
		}
		return "starting", "Starting QUIC relay listener…"
	}
	if m.OutboundRole {
		if m.UsingRelay {
			return "ok", "Registered with relay " + m.ActiveRelayAddr
		}
		if m.DialAttempts.Load() > 0 && m.DialOK.Load() == 0 {
			return "degraded", "No relay registered - check seeds, DNS, or P2P peers with NODE_DOGEGO_RELAY_CGNAT"
		}
		if len(m.discovery) == 0 {
			return "warming", "Discovering public relays…"
		}
		return "warming", "Dialing relays…"
	}
	return "off", "No DGR role active"
}

// Snapshot returns JSON-friendly metrics for /api/dgr and the P2P dashboard.
func (m *Metrics) Snapshot() map[string]any {
	if m == nil {
		return map[string]any{"enabled": false}
	}
	m.mu.RLock()
	clients := append([]clientRow(nil), m.clients...)
	outbound := append([]sessionRow(nil), m.outbound...)
	discovery := append([]string(nil), m.discovery...)
	enabled := m.Enabled
	inbound := m.InboundRole
	outboundRole := m.OutboundRole
	listen := m.ListenAddr
	listenBound := m.ListenBound
	advertise := m.AdvertiseAddr
	relayPort := m.RelayPort
	listenerOK := m.ListenerOK
	bitHex := m.ServiceBitHex
	using := m.UsingRelay
	active := m.ActiveRelayAddr
	activeCert := m.ActiveRelayCert
	serverCert := m.ServerCertSHA256
	inboundN := m.InboundSessions
	started := m.startedAt
	health, healthMsg := m.computeHealthLocked()
	m.mu.RUnlock()

	uptime := int64(time.Since(started).Seconds())
	return map[string]any{
		"enabled":              enabled,
		"inbound_relay":        inbound,
		"outbound_relay":       outboundRole,
		"listen":               listen,
		"listen_bound":         listenBound,
		"advertise_addr":       advertise,
		"relay_port":           relayPort,
		"listener_ok":          listenerOK,
		"health":               health,
		"health_message":       healthMsg,
		"service_bit_hex":      bitHex,
		"using_relay":          using,
		"active_relay":         active,
		"active_relay_cert_sha256": activeCert,
		"server_cert_sha256":   serverCert,
		"uptime_seconds":       uptime,
		"registered_clients":   len(clients),
		"inbound_sessions":     inboundN,
		"outbound_sessions":    len(outbound),
		"discovery_targets":    discovery,
		"register_ok":          m.RegisterOK.Load(),
		"register_fail":        m.RegisterFail.Load(),
		"dial_attempts":        m.DialAttempts.Load(),
		"dial_ok":              m.DialOK.Load(),
		"dial_fail":            m.DialFail.Load(),
		"frames_in":            m.FramesIn.Load(),
		"frames_out":           m.FramesOut.Load(),
		"pings":                m.Pings.Load(),
		"pongs":                m.Pongs.Load(),
		"inv_tx_in":            m.InvTxIn.Load(),
		"inv_tx_out":           m.InvTxOut.Load(),
		"p2p_frames_in":        m.P2PFramesIn.Load(),
		"p2p_frames_out":       m.P2PFramesOut.Load(),
		"p2p_proxy_ok":         m.P2PProxyOK.Load(),
		"p2p_proxy_fail":       m.P2PProxyFail.Load(),
		"p2p_publish_in":       m.P2PPublishIn.Load(),
		"p2p_publish_out":      m.P2PPublishOut.Load(),
		"p2p_push_in":          m.P2PPushIn.Load(),
		"p2p_push_out":         m.P2PPushOut.Load(),
		"p2p_tunnel_in":        m.P2PTunnelIn.Load(),
		"p2p_tunnel_out":       m.P2PTunnelOut.Load(),
		"peer_hints_in":        m.PeerHintsIn.Load(),
		"peer_hints_out":       m.PeerHintsOut.Load(),
		"tls_pin_ok":           m.TLSPinOK.Load(),
		"tls_pin_fail":         m.TLSPinFail.Load(),
		"rate_limited":         m.RateLimited.Load(),
		"bytes_in":             m.BytesIn.Load(),
		"bytes_out":            m.BytesOut.Load(),
		"clients":              clients,
		"outbound":             outbound,
	}
}
