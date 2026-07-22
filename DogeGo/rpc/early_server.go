// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"dogego/httptls"
)

const rpcWarmupCode = -28 // RPC_IN_WARMUP (Core-compatible)

const rpcWarmupMessage = "RPC warming up; node initializing (sync continues in background)"

// EarlyServer binds JSON-RPC immediately and serves a warmup response until Activate wires the full handler.
type EarlyServer struct {
	mu      sync.RWMutex
	handler http.Handler
	ready   atomic.Bool
}

// Ready reports whether the full RPC handler has been activated.
func (s *EarlyServer) Ready() bool {
	return s.ready.Load()
}

// Activate swaps in the production JSON-RPC handler (without auth wrapping; auth is on the outer listener).
func (s *EarlyServer) Activate(h http.Handler) {
	if s == nil || h == nil {
		return
	}
	s.mu.Lock()
	s.handler = h
	s.mu.Unlock()
	s.ready.Store(true)
}

func (s *EarlyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	h := s.handler
	s.mu.RUnlock()
	if h != nil {
		h.ServeHTTP(w, r)
		return
	}
	serveWarmupRPC(w, r)
}

// StartEarlyListen binds addr and serves JSON-RPC in a background goroutine.
// onStop is called when the listener exits (pass nil to ignore).
func StartEarlyListen(addr string, tls httptls.Pair, auth *RPCAuth, onStop func(error)) (*EarlyServer, error) {
	es := &EarlyServer{}
	ln, _, err := httptls.Listen(addr, tls)
	if err != nil {
		return nil, err
	}
	srv := &http.Server{
		Handler:           wrapIfAuth(auth, es),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		err := srv.Serve(ln)
		if onStop != nil {
			onStop(err)
		}
	}()
	return es, nil
}

func serveWarmupRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	trim := bytes.TrimSpace(body)
	if len(trim) > 0 && trim[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(body, &batch); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		out := make([]map[string]interface{}, len(batch))
		for i, rawMsg := range batch {
			out[i] = warmupResponse(parseRPCRequestID(rawMsg))
		}
		_ = json.NewEncoder(w).Encode(out)
		return
	}
	_ = json.NewEncoder(w).Encode(warmupResponse(parseRPCRequestID(body)))
}

func warmupResponse(id json.RawMessage) map[string]interface{} {
	if len(id) == 0 {
		id = json.RawMessage(`1`)
	}
	return map[string]interface{}{
		"jsonrpc": "1.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    rpcWarmupCode,
			"message": rpcWarmupMessage,
		},
	}
}

func parseRPCRequestID(body []byte) json.RawMessage {
	var req struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(body, &req); err != nil || len(req.ID) == 0 {
		return json.RawMessage(`1`)
	}
	return req.ID
}
