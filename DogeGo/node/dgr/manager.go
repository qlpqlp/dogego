// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dgr

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/config"
	"dogego/rpc"
)

// P2PBridge wires phase-4 bidirectional relay (client publish / operator fan-in).
type P2PBridge struct {
	// Publish relays a client-originated P2P message to the operator's Dogecoin peers.
	Publish func(cmd string, payload []byte) error
	// OnPush delivers operator-fanned P2P traffic to a CGNAT client.
	OnPush func(cmd string, payload []byte)
}

// P2PPeerSource returns P2P peers that may advertise NODE_DOGEGO_RELAY_CGNAT.
type P2PPeerSource func() []P2PRelayPeer

// Manager runs optional inbound relay server and/or outbound relay client.
type Manager struct {
	cfg     config.DogeGoRelayCGNAT
	metrics *Metrics

	server *relayServer
	client *relayClient

	bridge atomic.Pointer[P2PBridge]

	mu           sync.Mutex
	ctx          context.Context
	p2pMode      string
	network      string
	p2pPort      int
	peers        P2PPeerSource
	hooks        *P2PHooks
	learned      *LearnedRelayStore
	onSeedsMerge func([]string) // persist into dogecoinconf relay_seeds
}

// P2PHooks wires phase-2 DGR behavior (peer hints + P2P frame proxy).
type P2PHooks struct {
	Magic        [4]byte
	PeerHints    func() []string
	OnPeerHints  func([]string)
	RelayBook    func() RelayAddrBook
	OnTunnelPush func(peer string, wireMsg []byte)
	LearnedDir   string
	OnSeedsMerge func([]string)
}

// Start creates and starts DGR subsystems from effective config.
func Start(ctx context.Context, cfg config.DogeGoRelayCGNAT, p2pMode, network string, p2pPort int, peers P2PPeerSource, hooks *P2PHooks) (*Manager, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	m := &Manager{
		cfg:     cfg,
		metrics: newMetrics(),
		ctx:     ctx,
		p2pMode: p2pMode,
		network: network,
		p2pPort: p2pPort,
		peers:   peers,
		hooks:   hooks,
	}
	if hooks != nil {
		m.learned = OpenLearnedRelayStore(hooks.LearnedDir)
		m.onSeedsMerge = hooks.OnSeedsMerge
	} else {
		m.learned = OpenLearnedRelayStore("")
	}
	m.metrics.Enabled = true
	m.metrics.InboundRole = cfg.RoleInbound()
	m.metrics.OutboundRole = cfg.RoleOutbound(p2pMode)
	m.metrics.ListenAddr = cfg.EffectiveListen()
	m.metrics.RelayPort = cfg.EffectiveRelayPort()
	m.metrics.ServiceBitHex = rpc.FormatServicesHex(chain.ServiceDogeGoRelayCGNAT)

	if cfg.RoleInbound() {
		srvCfg := ServerConfig{
			Listen:               cfg.EffectiveListen(),
			Network:              network,
			AuthToken:            cfg.AuthToken,
			MaxClients:           cfg.EffectiveMaxClients(),
			AllowClients:         append([]string(nil), cfg.AllowClients...),
			P2PPort:              p2pPort,
			MaxSessionFramesPerS: cfg.EffectiveMaxSessionFramesPerSec(),
			MaxP2PProxyPerS:      cfg.EffectiveMaxP2PProxyPerSec(),
			MaxRegisterPerMin:    cfg.EffectiveMaxRegisterPerMin(),
		}
		if hooks != nil {
			srvCfg.P2PMagic = hooks.Magic
			srvCfg.PeerHints = hooks.PeerHints
		}
		srvCfg.OnClientPublish = m.handleClientPublish
		srv, err := startServer(ctx, srvCfg, m.metrics)
		if err != nil {
			return nil, fmt.Errorf("dgr server: %w", err)
		}
		m.server = srv
		m.metrics.mu.Lock()
		m.metrics.InboundSessions = 0
		m.metrics.mu.Unlock()
		applog.Line("dgr", "inbound relay operator: advertising NODE_DOGEGO_RELAY_CGNAT on P2P version/addr (Core ignores unknown service bits)")
	}
	if cfg.RoleOutbound(p2pMode) {
		if err := m.startOutboundClientLocked(ctx); err != nil {
			return nil, err
		}
	}
	go m.learnLoop(ctx)
	return m, nil
}

func (m *Manager) startOutboundClientLocked(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if m.client != nil {
		return nil
	}
	cfg := m.cfg
	var peerFn func() []P2PRelayPeer
	if m.peers != nil {
		peerFn = m.peers
	}
	clientCfg := ClientConfig{
		Network:      m.network,
		P2PPort:      m.p2pPort,
		AuthToken:    cfg.AuthToken,
		MaxConns:     cfg.EffectiveMaxRelayConns(),
		RelayPort:    cfg.EffectiveRelayPort(),
		StaticSeeds:  append([]string(nil), cfg.RelaySeeds...),
		RelayDNSSeed: cfg.RelayDNSSeed,
		RelayTLSPins: append([]string(nil), cfg.RelayTLSPins...),
		P2PPeers:     peerFn,
		LearnedSeeds: func() []string {
			if m.learned == nil {
				return nil
			}
			return m.learned.List()
		},
		OnLearnedPeer: func(quicHostPort string) {
			m.NoteLearnedRelay(quicHostPort)
		},
	}
	if m.hooks != nil {
		clientCfg.OnPeerHints = m.hooks.OnPeerHints
		clientCfg.RelayBook = m.hooks.RelayBook
		clientCfg.OnPush = m.handleClientPush
		clientCfg.OnTunnelPush = m.hooks.OnTunnelPush
	}
	m.client = startClient(ctx, clientCfg, m.metrics)
	m.metrics.OutboundRole = true
	applog.Line("dgr", fmt.Sprintf("outbound relay enabled (max %d, port %d)", cfg.EffectiveMaxRelayConns(), cfg.EffectiveRelayPort()))
	return nil
}

// EnsureOutboundClient starts the outbound DGR client if it is not already running.
// Used when a listening node still has no inbound peers after a grace period.
func (m *Manager) EnsureOutboundClient() bool {
	if m == nil || !m.cfg.Enabled {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client != nil {
		return true
	}
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if err := m.startOutboundClientLocked(ctx); err != nil {
		applog.Line("dgr", "auto outbound: "+err.Error())
		return false
	}
	applog.Line("dgr", "auto outbound: started client after no inbound peers (CGNAT/NAT likely)")
	return m.client != nil
}

// NoteLearnedRelay records a QUIC relay advertised by a DGR operator and merges into conf seeds.
func (m *Manager) NoteLearnedRelay(quicHostPort string) {
	if m == nil || m.learned == nil {
		return
	}
	if !m.learned.Note(quicHostPort, m.cfg.EffectiveRelayPort()) {
		return
	}
	applog.Line("dgr", "learned public relay "+quicHostPort)
	m.mergeSeedsToConf()
}

func (m *Manager) learnLoop(ctx context.Context) {
	if m == nil {
		return
	}
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()
	m.ingestLiveOperators()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.ingestLiveOperators()
		}
	}
}

func (m *Manager) ingestLiveOperators() {
	if m == nil || m.peers == nil {
		return
	}
	port := m.cfg.EffectiveRelayPort()
	var added []string
	for _, p := range m.peers() {
		if !chain.HasDogeGoRelayCGNAT(p.Services) {
			continue
		}
		host, _, err := splitHost(p.TCPAddr)
		if err != nil || host == "" {
			continue
		}
		hp := normalizeHostPort(host, port)
		if m.learned != nil && m.learned.Note(hp, port) {
			added = append(added, hp)
		}
	}
	if len(added) > 0 {
		applog.Line("dgr", fmt.Sprintf("learned %d DGR operator relay(s) from P2P handshake", len(added)))
		m.mergeSeedsToConf()
	}
}

func (m *Manager) mergeSeedsToConf() {
	if m == nil || m.onSeedsMerge == nil || m.learned == nil {
		return
	}
	merged := MergeRelaySeedLists(m.cfg.RelaySeeds, m.learned.List(), maxLearnedRelays)
	m.cfg.RelaySeeds = merged
	m.onSeedsMerge(merged)
	// Keep live client static seeds in sync when already running.
	m.mu.Lock()
	if m.client != nil {
		m.client.cfg.StaticSeeds = append([]string(nil), merged...)
	}
	m.mu.Unlock()
}

// LearnedRelays returns persisted operator QUIC targets.
func (m *Manager) LearnedRelays() []string {
	if m == nil || m.learned == nil {
		return nil
	}
	return m.learned.List()
}

// SetAdvertiseHost sets the public QUIC address for inbound rdogego (from UPnP or manual).
func (m *Manager) SetAdvertiseHost(host string, port int) {
	if m == nil || m.metrics == nil {
		return
	}
	if port <= 0 {
		port = m.cfg.EffectiveRelayPort()
	}
	m.metrics.SetAdvertiseHost(host, port)
}

// MetricsSnapshot returns live DGR metrics for dashboard/API.
func (m *Manager) MetricsSnapshot() map[string]any {
	if m == nil {
		return map[string]any{"enabled": false}
	}
	out := m.metrics.Snapshot()
	if learned := m.LearnedRelays(); len(learned) > 0 {
		out["learned_relays"] = learned
	}
	return out
}

// UsingRelay reports whether an outbound relay session is registered.
func (m *Manager) UsingRelay() bool {
	if m == nil || m.metrics == nil {
		return false
	}
	m.metrics.mu.RLock()
	using := m.metrics.UsingRelay
	m.metrics.mu.RUnlock()
	return using
}

// SetP2PBridge wires phase-4 publish (client→network) and push (network→client) handlers.
func (m *Manager) SetP2PBridge(b *P2PBridge) {
	if m == nil {
		return
	}
	if b == nil {
		m.bridge.Store(nil)
		return
	}
	m.bridge.Store(b)
}

// InboundRelay reports whether this node runs the public QUIC relay listener.
func (m *Manager) InboundRelay() bool {
	if m == nil {
		return false
	}
	return m.server != nil
}

// Publish sends a P2P message from a CGNAT client to the operator for network relay.
func (m *Manager) Publish(cmd string, payload []byte) bool {
	if m == nil || m.client == nil {
		return false
	}
	return m.client.publishP2P(cmd, payload)
}

// FanIn pushes a P2P message received by the operator to all registered CGNAT clients.
func (m *Manager) FanIn(cmd string, payload []byte) {
	if m == nil || m.server == nil {
		return
	}
	m.server.pushToClients(cmd, payload, 0)
}

func (m *Manager) handleClientPublish(cmd string, payload []byte) {
	if m == nil {
		return
	}
	b := m.bridge.Load()
	if b == nil || b.Publish == nil {
		return
	}
	if err := b.Publish(cmd, payload); err != nil {
		applog.Line("dgr", "client publish "+cmd+": "+err.Error())
	}
}

func (m *Manager) handleClientPush(cmd string, payload []byte) {
	if m == nil {
		return
	}
	b := m.bridge.Load()
	if b == nil || b.OnPush == nil {
		return
	}
	b.OnPush(cmd, payload)
}

// RelayInvTx forwards a tx inv payload through the outbound relay (best-effort).
func (m *Manager) RelayInvTx(payload []byte) bool {
	if m == nil || m.client == nil {
		return false
	}
	return m.client.RelayInvTx(payload)
}

// RelayP2PFrame proxies one P2P wire frame through the outbound relay (phase 2).
func (m *Manager) RelayP2PFrame(peer string, wireMsg []byte, timeout time.Duration) ([]byte, error) {
	if m == nil || m.client == nil {
		return nil, fmt.Errorf("dgr: outbound relay not active")
	}
	return m.client.RelayP2PFrame(peer, wireMsg, timeout)
}

// Close stops DGR listeners and client sessions.
func (m *Manager) Close() {
	if m == nil {
		return
	}
	if m.client != nil {
		m.client.close()
	}
	if m.server != nil {
		m.server.close()
	}
}

// AdvertiseRelayService reports whether NODE_DOGEGO_RELAY_CGNAT should be set on P2P version/addr.
func AdvertiseRelayService(cfg config.DogeGoRelayCGNAT) bool {
	return cfg.AdvertiseServiceBit()
}
