// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"dogego/applog"
	"github.com/quic-go/quic-go"
)

type relayServer struct {
	cfg     ServerConfig
	metrics *Metrics

	mu       sync.Mutex
	sessions map[uint64]*serverSession
	nextID   uint64
	register *registerLimiter
	pool     *p2pTunnelPool

	listener *quic.Listener
	udpConn  *net.UDPConn
}

type ServerConfig struct {
	Listen               string
	Network              string
	AuthToken            string
	MaxClients           int
	AllowClients         []string
	P2PPort              int
	P2PMagic             [4]byte
	PeerHints            func() []string
	OnClientPublish      func(cmd string, payload []byte)
	MaxSessionFramesPerS float64
	MaxP2PProxyPerS      float64
	MaxRegisterPerMin    int
}

type serverSession struct {
	id       uint64
	remote   string
	network  string
	p2pPort  int
	stream   quic.Stream
	since    time.Time
	last     time.Time
	framesIn atomic.Uint64
	framesOut atomic.Uint64
	frameLim *tokenBucket
	p2pLim   *tokenBucket
}

func startServer(ctx context.Context, cfg ServerConfig, metrics *Metrics) (*relayServer, error) {
	tlsConf, err := serverTLSConfig()
	if err != nil {
		return nil, err
	}
	addr, err := net.ResolveUDPAddr("udp", cfg.Listen)
	if err != nil {
		return nil, err
	}
	uc, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	qconf := &quic.Config{
		MaxIdleTimeout:     5 * time.Minute,
		KeepAlivePeriod:    25 * time.Second,
		MaxIncomingStreams: 512,
	}
	ln, err := quic.Listen(uc, tlsConf, qconf)
	if err != nil {
		_ = uc.Close()
		return nil, err
	}
	s := &relayServer{
		cfg:      cfg,
		metrics:  metrics,
		sessions: make(map[uint64]*serverSession),
		register: newRegisterLimiter(cfg.MaxRegisterPerMin),
		listener: ln,
		udpConn:  uc,
	}
	s.pool = newP2PTunnelPool(cfg.P2PMagic, s.pushTunnelData)
	if metrics != nil && uc.LocalAddr() != nil {
		metrics.mu.Lock()
		metrics.ListenBound = uc.LocalAddr().String()
		metrics.ListenerOK = true
		if fp, err := ServerCertFingerprint(); err == nil {
			metrics.ServerCertSHA256 = fp
		}
		metrics.mu.Unlock()
	}
	go s.acceptLoop(ctx)
	applog.Line("dgr", fmt.Sprintf("relay listening QUIC %s (max %d clients)", cfg.Listen, cfg.MaxClients))
	return s, nil
}

func (s *relayServer) acceptLoop(ctx context.Context) {
	for {
		conn, err := s.listener.Accept(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if ctx.Err() != nil {
				return
			}
			continue
		}
		go s.handleConn(ctx, conn)
	}
}

func (s *relayServer) handleConn(ctx context.Context, conn quic.Connection) {
	remote := conn.RemoteAddr().String()
	if !clientAllowed(conn.RemoteAddr(), s.cfg.AllowClients) {
		s.metrics.RegisterFail.Add(1)
		_ = conn.CloseWithError(1, "deny")
		return
	}
	regKey := registerLimiterKey(conn.RemoteAddr())
	if !s.register.allow(regKey) {
		s.metrics.RegisterFail.Add(1)
		s.metrics.RateLimited.Add(1)
		_ = conn.CloseWithError(1, "register-rate")
		return
	}
	stream, err := conn.AcceptStream(ctx)
	if err != nil {
		return
	}
	typ, payload, err := readFrame(stream)
	if err != nil || typ != MsgRegister {
		s.metrics.RegisterFail.Add(1)
		_ = stream.Close()
		_ = conn.CloseWithError(1, "register")
		return
	}
	network, token, p2pPort, ok := decodeRegister(payload)
	if !ok || (s.cfg.AuthToken != "" && token != s.cfg.AuthToken) {
		s.metrics.RegisterFail.Add(1)
		_ = stream.Close()
		_ = conn.CloseWithError(1, "auth")
		return
	}
	if network == "" {
		network = s.cfg.Network
	}
	if p2pPort <= 0 {
		p2pPort = s.cfg.P2PPort
	}
	s.mu.Lock()
	if len(s.sessions) >= s.cfg.MaxClients {
		s.mu.Unlock()
		s.metrics.RegisterFail.Add(1)
		_ = stream.Close()
		_ = conn.CloseWithError(1, "full")
		return
	}
	var idBuf [8]byte
	_, _ = rand.Read(idBuf[:])
	id := binary.BigEndian.Uint64(idBuf[:])
	if id == 0 {
		id = 1
	}
	sess := &serverSession{
		id: id, remote: remote, network: network, p2pPort: p2pPort,
		stream: stream, since: time.Now(), last: time.Now(),
		frameLim: newTokenBucket(s.cfg.MaxSessionFramesPerS, s.cfg.MaxSessionFramesPerS*2),
		p2pLim:   newTokenBucket(s.cfg.MaxP2PProxyPerS, s.cfg.MaxP2PProxyPerS*2),
	}
	s.sessions[id] = sess
	s.mu.Unlock()

	if err := writeFrame(stream, MsgRegisterOK, encodeRegisterOK(id)); err != nil {
		s.dropSession(id)
		return
	}
	s.metrics.RegisterOK.Add(1)
	s.metrics.noteClient(clientRow{
		Remote: remote, Network: network, P2PPort: p2pPort,
		Since: sess.since, LastActive: sess.last,
	})
	applog.Line("dgr", fmt.Sprintf("client registered %s network=%s p2p=%d session=%d", remote, network, p2pPort, id))
	s.sendPeerHints(sess)
	go s.sessionLoop(ctx, conn, sess)
}

func (s *relayServer) sendPeerHints(sess *serverSession) {
	if s.cfg.PeerHints == nil {
		return
	}
	hints := s.cfg.PeerHints()
	if len(hints) == 0 {
		return
	}
	if err := writeFrame(sess.stream, MsgPeerHint, encodePeerHint(hints...)); err != nil {
		return
	}
	s.metrics.PeerHintsOut.Add(1)
	s.metrics.FramesOut.Add(1)
	s.metrics.BytesOut.Add(uint64(9 + len(encodePeerHint(hints...))))
	sess.framesOut.Add(1)
}

func (s *relayServer) sessionLoop(ctx context.Context, conn quic.Connection, sess *serverSession) {
	defer s.dropSession(sess.id)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = sess.stream.SetReadDeadline(time.Now().Add(45 * time.Second))
		typ, payload, err := readFrame(sess.stream)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				s.metrics.Pings.Add(1)
				_ = writeFrame(sess.stream, MsgPing, nil)
				continue
			}
			return
		}
		sess.last = time.Now()
		if typ != MsgPing && typ != MsgPong && !sess.frameLim.allow() {
			s.metrics.RateLimited.Add(1)
			continue
		}
		s.metrics.FramesIn.Add(1)
		s.metrics.BytesIn.Add(uint64(9 + len(payload)))
		sess.framesIn.Add(1)
		switch typ {
		case MsgPing:
			s.metrics.Pongs.Add(1)
			_ = writeFrame(sess.stream, MsgPong, nil)
		case MsgPong:
			s.metrics.Pongs.Add(1)
		case MsgInvTx:
			s.metrics.InvTxIn.Add(1)
			s.handleClientPublish("inv", payload)
		case MsgP2PPublish:
			s.handleClientPublishFrame(payload)
		case MsgP2PFrame:
			s.handleP2PFrame(sess, payload)
		default:
		}
		s.metrics.noteClient(clientRow{
			Remote: sess.remote, Network: sess.network, P2PPort: sess.p2pPort,
			Since: sess.since, LastActive: sess.last,
			FramesIn: sess.framesIn.Load(), FramesOut: sess.framesOut.Load(),
		})
	}
}

func (s *relayServer) handleClientPublishFrame(payload []byte) {
	cmd, body, ok := decodeP2PPayload(payload)
	if !ok {
		return
	}
	s.handleClientPublish(cmd, body)
}

func (s *relayServer) handleClientPublish(cmd string, payload []byte) {
	s.metrics.P2PPublishIn.Add(1)
	if s.cfg.OnClientPublish != nil {
		s.cfg.OnClientPublish(cmd, payload)
	}
}

// pushToClients fans a P2P message to registered CGNAT clients (excludeSession=0 → all).
func (s *relayServer) pushToClients(cmd string, payload []byte, excludeSession uint64) {
	if s == nil || cmd == "" {
		return
	}
	frame, err := encodeP2PPayload(cmd, payload)
	if err != nil {
		return
	}
	s.mu.Lock()
	sessions := make([]*serverSession, 0, len(s.sessions))
	for id, sess := range s.sessions {
		if excludeSession != 0 && id == excludeSession {
			continue
		}
		sessions = append(sessions, sess)
	}
	s.mu.Unlock()
	for _, sess := range sessions {
		if !sess.frameLim.allow() {
			s.metrics.RateLimited.Add(1)
			continue
		}
		if err := writeFrame(sess.stream, MsgP2PPush, frame); err != nil {
			continue
		}
		s.metrics.P2PPushOut.Add(1)
		s.metrics.FramesOut.Add(1)
		s.metrics.BytesOut.Add(uint64(9 + len(frame)))
		sess.framesOut.Add(1)
	}
}

func (s *relayServer) handleP2PFrame(sess *serverSession, payload []byte) {
	if !sess.p2pLim.allow() {
		s.metrics.RateLimited.Add(1)
		s.metrics.P2PProxyFail.Add(1)
		return
	}
	reqID, peer, wireMsg, ok := decodeP2PFrameRequest(payload)
	if !ok {
		s.metrics.P2PProxyFail.Add(1)
		return
	}
	s.metrics.P2PFramesIn.Add(1)
	respWire, status := s.pool.Proxy(sess.id, peer, wireMsg)
	respPayload, err := encodeP2PFrameResponse(reqID, status, respWire)
	if err != nil {
		s.metrics.P2PProxyFail.Add(1)
		return
	}
	if status == P2PProxyOK {
		s.metrics.P2PProxyOK.Add(1)
		s.metrics.P2PFramesOut.Add(1)
	} else if status == P2PProxyNoResponse {
		s.metrics.P2PProxyOK.Add(1)
	} else {
		s.metrics.P2PProxyFail.Add(1)
	}
	if err := writeFrame(sess.stream, MsgP2PFrame, respPayload); err != nil {
		return
	}
	s.metrics.FramesOut.Add(1)
	s.metrics.BytesOut.Add(uint64(9 + len(respPayload)))
	sess.framesOut.Add(1)
}

func (s *relayServer) pushTunnelData(sessionID uint64, peer string, wireMsg []byte) {
	if s == nil || len(wireMsg) == 0 {
		return
	}
	payload, err := encodeTunnelData(peer, wireMsg)
	if err != nil {
		return
	}
	s.mu.Lock()
	sess := s.sessions[sessionID]
	s.mu.Unlock()
	if sess == nil {
		return
	}
	if !sess.frameLim.allow() {
		s.metrics.RateLimited.Add(1)
		return
	}
	if err := writeFrame(sess.stream, MsgP2PTunnel, payload); err != nil {
		return
	}
	s.metrics.P2PTunnelOut.Add(1)
	s.metrics.P2PFramesOut.Add(1)
	s.metrics.FramesOut.Add(1)
	s.metrics.BytesOut.Add(uint64(9 + len(payload)))
	sess.framesOut.Add(1)
}

func (s *relayServer) dropSession(id uint64) {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	if ok {
		delete(s.sessions, id)
	}
	n := len(s.sessions)
	s.mu.Unlock()
	if !ok {
		return
	}
	if s.pool != nil {
		s.pool.dropSession(id)
	}
	s.metrics.dropClient(sess.remote)
	s.metrics.mu.Lock()
	s.metrics.InboundSessions = n
	s.metrics.mu.Unlock()
	_ = sess.stream.Close()
}

func (s *relayServer) close() {
	if s.listener != nil {
		_ = s.listener.Close()
	}
	if s.udpConn != nil {
		_ = s.udpConn.Close()
	}
}

func registerLimiterKey(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}
