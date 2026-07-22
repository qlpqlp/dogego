// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"dogego/applog"
	"dogego/chain"
	"github.com/quic-go/quic-go"
)

type relayClient struct {
	cfg     ClientConfig
	metrics *Metrics

	mu       sync.Mutex
	sessions map[string]*clientSession

	nextReqID    atomic.Uint32
	pending      map[uint32]chan p2pFrameResult
	refreshCount atomic.Uint64
}

type ClientConfig struct {
	Network       string
	P2PPort       int
	AuthToken     string
	MaxConns      int
	RelayPort     int
	StaticSeeds   []string
	LearnedSeeds  func() []string
	RelayDNSSeed  string
	RelayTLSPins  []string
	P2PPeers      func() []P2PRelayPeer
	RelayBook     func() RelayAddrBook
	OnPeerHints   func([]string)
	OnPush        func(cmd string, payload []byte)
	OnTunnelPush  func(peer string, wireMsg []byte)
	OnLearnedPeer func(quicHostPort string)
}

type p2pFrameWaiter struct {
	ch chan p2pFrameResult
}

type p2pFrameResult struct {
	status  byte
	wireMsg []byte
}

type clientSession struct {
	addr       string
	conn       quic.Connection
	stream     quic.Stream
	registered atomic.Bool
	since      time.Time
	last       time.Time
	framesIn   atomic.Uint64
	framesOut  atomic.Uint64
}

func startClient(ctx context.Context, cfg ClientConfig, metrics *Metrics) *relayClient {
	c := &relayClient{
		cfg:      cfg,
		metrics:  metrics,
		sessions: make(map[string]*clientSession),
		pending:  make(map[uint32]chan p2pFrameResult),
	}
	go c.maintainLoop(ctx)
	return c
}

func (c *relayClient) maintainLoop(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	c.refresh(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refresh(ctx)
		}
	}
}

func (c *relayClient) refresh(ctx context.Context) {
	var p2pPeers []P2PRelayPeer
	if c.cfg.P2PPeers != nil {
		p2pPeers = c.cfg.P2PPeers()
	}
	var learned []string
	if c.cfg.LearnedSeeds != nil {
		learned = c.cfg.LearnedSeeds()
	}
	targets := DiscoverTargets(ctx, c.cfg.RelayDNSSeed, c.cfg.StaticSeeds, learned, c.cfg.RelayPort, p2pPeers)
	// Persist live operator advertisements into the learned store / conf path.
	if c.cfg.OnLearnedPeer != nil {
		for _, p := range p2pPeers {
			if !hasRelayService(p.Services) {
				continue
			}
			host, _, err := splitHost(p.TCPAddr)
			if err != nil || host == "" {
				continue
			}
			c.cfg.OnLearnedPeer(normalizeHostPort(host, c.cfg.RelayPort))
		}
	}
	// Soft demote recent failures, then crypto-shuffle so operators cannot predict
	// which public relay a client will use (privacy + load spread).
	if book := c.relayBook(); book != nil {
		targets = preferHealthyTargets(targets, book)
	}
	ShuffleSecure(targets)
	c.metrics.setDiscovery(targets)
	c.mu.Lock()
	active := len(c.sessions)
	sessionAddrs := make([]string, 0, active)
	for addr := range c.sessions {
		sessionAddrs = append(sessionAddrs, addr)
	}
	c.mu.Unlock()
	need := c.cfg.MaxConns - active
	// Periodic rotation (~5 min): when full and more targets exist, drop one random
	// session so the client does not stick forever to a single operator.
	n := c.refreshCount.Add(1)
	if need <= 0 && len(targets) > active && active > 0 && len(sessionAddrs) > 0 && n%15 == 0 {
		drop := sessionAddrs[secureIntn(len(sessionAddrs))]
		c.dropSession(drop)
		need = 1
	}
	if need <= 0 {
		return
	}
	for _, addr := range targets {
		if need <= 0 {
			break
		}
		c.mu.Lock()
		_, exists := c.sessions[addr]
		c.mu.Unlock()
		if exists {
			continue
		}
		c.metrics.DialAttempts.Add(1)
		if c.dialOne(ctx, addr) {
			c.metrics.DialOK.Add(1)
			need--
		} else {
			c.metrics.DialFail.Add(1)
			if book := c.relayBook(); book != nil {
				book.NoteFailure(addr)
			}
		}
	}
}

func hasRelayService(services uint64) bool {
	return chain.HasDogeGoRelayCGNAT(services)
}

func splitHost(tcpAddr string) (string, string, error) {
	return net.SplitHostPort(tcpAddr)
}

func preferHealthyTargets(targets []string, book RelayAddrBook) []string {
	if len(targets) < 2 || book == nil {
		return targets
	}
	healthy := make([]string, 0, len(targets))
	weak := make([]string, 0)
	for _, t := range targets {
		if book.RelayDialScore(t) < 0 {
			weak = append(weak, t)
			continue
		}
		healthy = append(healthy, t)
	}
	if len(healthy) == 0 {
		return targets
	}
	return append(healthy, weak...)
}

func (c *relayClient) relayBook() RelayAddrBook {
	if c == nil || c.cfg.RelayBook == nil {
		return nil
	}
	return c.cfg.RelayBook()
}

func (c *relayClient) dialOne(ctx context.Context, addr string) bool {
	if book := c.relayBook(); book != nil {
		book.NoteTry(addr)
	}
	tlsConf, err := clientTLSConfigWithPins(c.cfg.RelayTLSPins)
	if err != nil {
		return false
	}
	dialCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	conn, err := quic.DialAddr(dialCtx, addr, tlsConf, &quic.Config{
		MaxIdleTimeout:  5 * time.Minute,
		KeepAlivePeriod: 25 * time.Second,
	})
	if err != nil {
		if len(normalizeTLSPins(c.cfg.RelayTLSPins)) > 0 {
			c.metrics.TLSPinFail.Add(1)
		}
		return false
	}
	if len(normalizeTLSPins(c.cfg.RelayTLSPins)) > 0 {
		c.metrics.TLSPinOK.Add(1)
	}
	var peerCert string
	if st := conn.ConnectionState().TLS; len(st.PeerCertificates) > 0 {
		peerCert = certFingerprint(st.PeerCertificates[0])
	}
	stream, err := conn.OpenStreamSync(dialCtx)
	if err != nil {
		_ = conn.CloseWithError(1, "stream")
		return false
	}
	payload := encodeRegister(c.cfg.Network, c.cfg.AuthToken, c.cfg.P2PPort)
	if err := writeFrame(stream, MsgRegister, payload); err != nil {
		_ = stream.Close()
		_ = conn.CloseWithError(1, "register")
		return false
	}
	typ, resp, err := readFrame(stream)
	if err != nil || typ != MsgRegisterOK {
		_ = stream.Close()
		_ = conn.CloseWithError(1, "register")
		return false
	}
	if _, ok := decodeRegisterOK(resp); !ok {
		_ = stream.Close()
		_ = conn.CloseWithError(1, "register")
		return false
	}
	sess := &clientSession{
		addr: addr, conn: conn, stream: stream,
		since: time.Now(), last: time.Now(),
	}
	sess.registered.Store(true)
	c.mu.Lock()
	c.sessions[addr] = sess
	c.mu.Unlock()
	if book := c.relayBook(); book != nil {
		book.NoteSuccess(addr)
	}
	c.metrics.noteOutbound(sessionRow{
		Addr: addr, Since: sess.since, LastActive: sess.last, Registered: true,
	})
	if peerCert != "" {
		c.metrics.mu.Lock()
		c.metrics.ActiveRelayCert = peerCert
		c.metrics.mu.Unlock()
	}
	applog.Line("dgr", fmt.Sprintf("connected relay %s", addr))
	go c.readLoop(ctx, sess)
	return true
}

func (c *relayClient) readLoop(ctx context.Context, sess *clientSession) {
	defer c.dropSession(sess.addr)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		sess.stream.SetReadDeadline(time.Now().Add(60 * time.Second))
		typ, payload, err := readFrame(sess.stream)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if err == io.EOF {
				return
			}
			return
		}
		sess.last = time.Now()
		c.metrics.FramesIn.Add(1)
		c.metrics.BytesIn.Add(uint64(9 + len(payload)))
		sess.framesIn.Add(1)
		switch typ {
		case MsgPing:
			c.metrics.Pings.Add(1)
			_ = writeFrame(sess.stream, MsgPong, nil)
		case MsgPong:
			c.metrics.Pongs.Add(1)
		case MsgPeerHint:
			hints := decodePeerHints(payload)
			if len(hints) > 0 {
				c.metrics.PeerHintsIn.Add(1)
				if c.cfg.OnPeerHints != nil {
					c.cfg.OnPeerHints(hints)
				}
			}
		case MsgInvTx:
			c.metrics.InvTxIn.Add(1)
			c.deliverPush("inv", payload)
		case MsgP2PPush:
			c.metrics.P2PPushIn.Add(1)
			c.deliverPushFrame(payload)
		case MsgP2PTunnel:
			c.deliverTunnelPush(payload)
		case MsgP2PFrame:
			c.deliverP2PFrameResponse(payload)
		}
		c.metrics.noteOutbound(sessionRow{
			Addr: sess.addr, Since: sess.since, LastActive: sess.last, Registered: sess.registered.Load(),
			FramesIn: sess.framesIn.Load(), FramesOut: sess.framesOut.Load(),
		})
	}
}

func (c *relayClient) dropSession(addr string) {
	c.mu.Lock()
	sess, ok := c.sessions[addr]
	if ok {
		delete(c.sessions, addr)
	}
	c.mu.Unlock()
	if !ok {
		return
	}
	c.metrics.dropOutbound(addr)
	_ = sess.stream.Close()
	_ = sess.conn.CloseWithError(0, "done")
}

// RelayInvTx forwards a tx inv hash to the active outbound relay session (best-effort).
func (c *relayClient) RelayInvTx(payload []byte) bool {
	return c.publishP2P("inv", payload)
}

func (c *relayClient) deliverPushFrame(payload []byte) {
	cmd, body, ok := decodeP2PPayload(payload)
	if !ok {
		return
	}
	c.deliverPush(cmd, body)
}

func (c *relayClient) deliverPush(cmd string, payload []byte) {
	if c.cfg.OnPush != nil {
		c.cfg.OnPush(cmd, payload)
	}
}

func (c *relayClient) deliverTunnelPush(payload []byte) {
	peer, wireMsg, ok := decodeTunnelData(payload)
	if !ok || len(wireMsg) == 0 {
		return
	}
	c.metrics.P2PTunnelIn.Add(1)
	if c.cfg.OnTunnelPush != nil {
		c.cfg.OnTunnelPush(peer, wireMsg)
	}
}

// publishP2P sends a P2P message to the operator for network relay (phase 4).
func (c *relayClient) publishP2P(cmd string, payload []byte) bool {
	if c == nil || len(payload) == 0 {
		return false
	}
	frame, err := encodeP2PPayload(cmd, payload)
	if err != nil {
		return false
	}
	c.mu.Lock()
	var pick *clientSession
	for _, s := range c.sessions {
		if s.registered.Load() {
			pick = s
			break
		}
	}
	c.mu.Unlock()
	if pick == nil {
		return false
	}
	if err := writeFrame(pick.stream, MsgP2PPublish, frame); err != nil {
		return false
	}
	c.metrics.P2PPublishOut.Add(1)
	c.metrics.InvTxOut.Add(1)
	c.metrics.FramesOut.Add(1)
	c.metrics.BytesOut.Add(uint64(9 + len(frame)))
	pick.framesOut.Add(1)
	return true
}

func (c *relayClient) deliverP2PFrameResponse(payload []byte) {
	reqID, status, wireMsg, ok := decodeP2PFrameResponse(payload)
	if !ok {
		return
	}
	c.metrics.P2PFramesIn.Add(1)
	if status == P2PProxyOK {
		c.metrics.P2PFramesOut.Add(1)
	}
	c.mu.Lock()
	ch := c.pending[reqID]
	if ch != nil {
		delete(c.pending, reqID)
	}
	c.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- p2pFrameResult{status: status, wireMsg: wireMsg}:
	default:
	}
}

// RelayP2PFrame sends one proxied P2P message through the active relay session.
func (c *relayClient) RelayP2PFrame(peer string, wireMsg []byte, timeout time.Duration) ([]byte, error) {
	if c == nil || len(wireMsg) == 0 {
		return nil, fmt.Errorf("dgr: invalid p2p frame request")
	}
	if timeout <= 0 {
		timeout = 50 * time.Second
	}
	c.mu.Lock()
	var pick *clientSession
	for _, s := range c.sessions {
		if s.registered.Load() {
			pick = s
			break
		}
	}
	c.mu.Unlock()
	if pick == nil {
		return nil, fmt.Errorf("dgr: no relay session")
	}
	reqID := c.nextReqID.Add(1)
	if reqID == 0 {
		reqID = c.nextReqID.Add(1)
	}
	payload, err := encodeP2PFrameRequest(reqID, peer, wireMsg)
	if err != nil {
		return nil, err
	}
	waitCh := make(chan p2pFrameResult, 1)
	c.mu.Lock()
	c.pending[reqID] = waitCh
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, reqID)
		c.mu.Unlock()
	}()
	if err := writeFrame(pick.stream, MsgP2PFrame, payload); err != nil {
		return nil, err
	}
	c.metrics.P2PFramesOut.Add(1)
	c.metrics.FramesOut.Add(1)
	c.metrics.BytesOut.Add(uint64(9 + len(payload)))
	pick.framesOut.Add(1)
	select {
	case res := <-waitCh:
		if res.status == P2PProxyNoResponse {
			return nil, nil
		}
		if res.status != P2PProxyOK {
			return nil, fmt.Errorf("dgr: p2p proxy %s", p2pProxyStatusText(res.status))
		}
		return res.wireMsg, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("dgr: p2p proxy timeout")
	}
}

func (c *relayClient) close() {
	c.mu.Lock()
	addrs := make([]string, 0, len(c.sessions))
	for a := range c.sessions {
		addrs = append(addrs, a)
	}
	c.mu.Unlock()
	for _, a := range addrs {
		c.dropSession(a)
	}
}
