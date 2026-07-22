// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package zmqnotify implements Core-compatible ZMQ PUB notifications (hashblock, hashtx, rawblock, rawtx).
package zmqnotify

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"

	"dogego/applog"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"

	zmq4 "github.com/go-zeromq/zmq4"
)

// Config holds bind endpoints (Core -zmqpub*); use tcp://host:port or host:port.
type Config struct {
	PubHashBlock string
	PubHashTx    string
	PubRawBlock  string
	PubRawTx     string
}

func (c Config) Enabled() bool {
	return c.PubHashBlock != "" || c.PubHashTx != "" || c.PubRawBlock != "" || c.PubRawTx != ""
}

// Hub publishes validation events on ZeroMQ PUB sockets (multipart: command, payload, LE seq).
type Hub struct {
	cfg Config

	ctx    context.Context
	cancel context.CancelFunc

	mu   sync.Mutex
	seq  uint32
	sock map[string]zmq4.Socket
}

// Start binds PUB sockets for each configured endpoint (shared per address like Core).
func Start(parent context.Context, cfg Config) (*Hub, error) {
	if !cfg.Enabled() {
		return nil, nil
	}
	ctx, cancel := context.WithCancel(parent)
	h := &Hub{cfg: cfg, ctx: ctx, cancel: cancel, sock: make(map[string]zmq4.Socket)}
	var firstErr error
	bind := func(addr string) error {
		ep, err := normalizeEndpoint(addr)
		if err != nil {
			return err
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.sock[ep]; ok {
			return nil
		}
		pub := zmq4.NewPub(ctx)
		if err := pub.Listen(ep); err != nil {
			_ = pub.Close()
			return fmt.Errorf("zmq listen %s: %w", ep, err)
		}
		h.sock[ep] = pub
		applog.Line("zmq", "listening on "+ep)
		return nil
	}
	for _, spec := range []struct {
		addr string
		need bool
	}{
		{cfg.PubHashBlock, true},
		{cfg.PubHashTx, true},
		{cfg.PubRawBlock, true},
		{cfg.PubRawTx, true},
	} {
		if spec.addr == "" {
			continue
		}
		if err := bind(spec.addr); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		h.Stop()
		return nil, firstErr
	}
	return h, nil
}

// Stop closes all ZMQ sockets.
func (h *Hub) Stop() {
	if h == nil {
		return
	}
	h.cancel()
	h.mu.Lock()
	defer h.mu.Unlock()
	for ep, s := range h.sock {
		_ = s.Close()
		delete(h.sock, ep)
	}
}

func normalizeEndpoint(addr string) (string, error) {
	a := strings.TrimSpace(addr)
	if a == "" {
		return "", fmt.Errorf("empty zmq address")
	}
	if !strings.Contains(a, "://") {
		a = "tcp://" + a
	}
	return a, nil
}

func (h *Hub) publish(addr, command string, payload []byte) {
	if h == nil || addr == "" {
		return
	}
	ep, err := normalizeEndpoint(addr)
	if err != nil {
		return
	}
	h.mu.Lock()
	s, ok := h.sock[ep]
	h.mu.Unlock()
	if !ok {
		return
	}
	h.mu.Lock()
	seq := h.seq
	h.seq++
	h.mu.Unlock()
	var seqBuf [4]byte
	binary.LittleEndian.PutUint32(seqBuf[:], seq)
	msg := zmq4.NewMsgFrom([]byte(command), payload, seqBuf[:])
	if err := s.SendMulti(msg); err != nil {
		applog.Line("zmq", fmt.Sprintf("publish %s: %v", command, err))
	}
}

// NotifyBlockAt publishes hashblock/rawblock for a connected height (reads journal + raw store).
func (h *Hub) NotifyBlockAt(j *store.HeaderJournal, raw *store.RawBlockStore, height int64) {
	if h == nil || j == nil || raw == nil || height < 0 {
		return
	}
	h80, err := j.ReadHeaderAt(height)
	if err != nil || len(h80) != 80 {
		return
	}
	blockHash := pow.BlockHashLE(h80)
	if h.cfg.PubHashBlock != "" {
		h.publish(h.cfg.PubHashBlock, "hashblock", hash32Wire(blockHash))
	}
	if h.cfg.PubRawBlock != "" {
		payload, err := raw.Get(blockHash)
		if err == nil && len(payload) > 0 {
			h.publish(h.cfg.PubRawBlock, "rawblock", payload)
		}
	}
}

// NotifyTx publishes hashtx/rawtx for a mempool transaction (Core NotifyTransaction).
func (h *Hub) NotifyTx(raw []byte) {
	if h == nil || len(raw) == 0 {
		return
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return
	}
	txid := tx.TxHash()
	if h.cfg.PubHashTx != "" {
		h.publish(h.cfg.PubHashTx, "hashtx", hash32Wire(txid))
	}
	if h.cfg.PubRawTx != "" {
		h.publish(h.cfg.PubRawTx, "rawtx", raw)
	}
}

// hash32Wire reverses internal hash byte order to match Core ZMQ (display order).
func hash32Wire(h [32]byte) []byte {
	out := make([]byte, 32)
	for i := 0; i < 32; i++ {
		out[31-i] = h[i]
	}
	return out
}
