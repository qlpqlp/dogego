// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package httptls provides optional TLS listeners for DogeGo HTTP servers (RPC and web UI).
package httptls

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

// Pair holds PEM certificate and private key paths (both required when either is set).
type Pair struct {
	CertFile string
	KeyFile  string
}

// Enabled reports whether TLS termination is requested.
func (p Pair) Enabled() bool {
	return strings.TrimSpace(p.CertFile) != "" || strings.TrimSpace(p.KeyFile) != ""
}

// Validate checks that cert and key paths exist when TLS is enabled.
func (p Pair) Validate() error {
	cert := strings.TrimSpace(p.CertFile)
	key := strings.TrimSpace(p.KeyFile)
	if cert == "" && key == "" {
		return nil
	}
	if cert == "" || key == "" {
		return fmt.Errorf("tls: both cert and key paths are required")
	}
	if _, err := os.Stat(cert); err != nil {
		return fmt.Errorf("tls cert %q: %w", cert, err)
	}
	if _, err := os.Stat(key); err != nil {
		return fmt.Errorf("tls key %q: %w", key, err)
	}
	return nil
}

// Listen binds addr on TCP. When p is enabled, returns a TLS listener (TLS 1.2+).
// scheme is "https" or "http" for building dashboard URLs.
//
// When host is "localhost", binds 127.0.0.1 and ::1 (when available) so both
// https://localhost:PORT and https://127.0.0.1:PORT work across Windows/Linux/macOS.
func Listen(addr string, p Pair) (ln net.Listener, scheme string, err error) {
	if err := p.Validate(); err != nil {
		return nil, "", err
	}
	host, port, splitErr := net.SplitHostPort(strings.TrimSpace(addr))
	if splitErr == nil && strings.EqualFold(strings.Trim(host, "[]"), "localhost") {
		return listenLocalhost(port, p)
	}
	return listenOne(addr, p)
}

func listenOne(addr string, p Pair) (net.Listener, string, error) {
	if !p.Enabled() {
		ln, err := net.Listen("tcp", addr)
		return ln, "http", err
	}
	cert, err := tls.LoadX509KeyPair(strings.TrimSpace(p.CertFile), strings.TrimSpace(p.KeyFile))
	if err != nil {
		return nil, "", fmt.Errorf("tls load key pair: %w", err)
	}
	tlsLn, err := tls.Listen("tcp", addr, &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
	return tlsLn, "https", err
}

func listenLocalhost(port string, p Pair) (net.Listener, string, error) {
	// Ephemeral port: a single bind only (IPv4+IPv6 would get different ports).
	if port == "0" {
		return listenOne(net.JoinHostPort("127.0.0.1", "0"), p)
	}
	candidates := []string{
		net.JoinHostPort("127.0.0.1", port),
		net.JoinHostPort("::1", port),
	}
	var lns []net.Listener
	var scheme string
	var lastErr error
	for _, a := range candidates {
		ln, sch, err := listenOne(a, p)
		if err != nil {
			lastErr = err
			continue
		}
		scheme = sch
		lns = append(lns, ln)
	}
	if len(lns) == 0 {
		if lastErr == nil {
			lastErr = fmt.Errorf("tls: could not bind localhost on port %s", port)
		}
		return nil, "", lastErr
	}
	if len(lns) == 1 {
		return lns[0], scheme, nil
	}
	return newMultiListener(lns), scheme, nil
}

// multiListener Accepts from several underlying listeners (IPv4 + IPv6 loopback).
type multiListener struct {
	lns    []net.Listener
	addr   net.Addr
	connCh chan acceptResult
	once   sync.Once
	closed chan struct{}
}

type acceptResult struct {
	c   net.Conn
	err error
}

func newMultiListener(lns []net.Listener) *multiListener {
	m := &multiListener{
		lns:    lns,
		addr:   lns[0].Addr(),
		connCh: make(chan acceptResult),
		closed: make(chan struct{}),
	}
	for _, ln := range lns {
		go m.acceptLoop(ln)
	}
	return m
}

func (m *multiListener) acceptLoop(ln net.Listener) {
	for {
		c, err := ln.Accept()
		select {
		case <-m.closed:
			if c != nil {
				_ = c.Close()
			}
			return
		case m.connCh <- acceptResult{c: c, err: err}:
			if err != nil {
				return
			}
		}
	}
}

func (m *multiListener) Accept() (net.Conn, error) {
	select {
	case <-m.closed:
		return nil, net.ErrClosed
	case r := <-m.connCh:
		return r.c, r.err
	}
}

func (m *multiListener) Close() error {
	var first error
	m.once.Do(func() {
		close(m.closed)
		for _, ln := range m.lns {
			if err := ln.Close(); err != nil && first == nil {
				first = err
			}
		}
	})
	return first
}

func (m *multiListener) Addr() net.Addr { return m.addr }
