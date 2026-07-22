// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"net/http"
	"sync"
	"time"
)

const (
	defaultAuthFailsPerMinute = 30
	rateLimitWindow           = 60 * time.Second
)

// RPCLimits configures optional per-IP JSON-RPC rate limits (0 = disabled).
type RPCLimits struct {
	// RequestsPerMinute caps total POST requests per client IP per minute (0 = off).
	RequestsPerMinute int
	// AuthFailsPerMinute caps failed HTTP Basic attempts per IP per minute (0 = default when auth on).
	AuthFailsPerMinute int
}

func (l RPCLimits) authFailCap(authEnabled bool) int {
	if !authEnabled {
		return 0
	}
	if l.AuthFailsPerMinute < 0 {
		return 0
	}
	if l.AuthFailsPerMinute > 0 {
		return l.AuthFailsPerMinute
	}
	return defaultAuthFailsPerMinute
}

type ipCounter struct {
	mu    sync.Mutex
	wins  map[string][]time.Time
	limit int
}

func newIPCounter(limit int) *ipCounter {
	if limit <= 0 {
		return nil
	}
	return &ipCounter{wins: make(map[string][]time.Time), limit: limit}
}

func (c *ipCounter) allow(key string) bool {
	if c == nil || c.limit <= 0 {
		return true
	}
	now := time.Now()
	cutoff := now.Add(-rateLimitWindow)
	c.mu.Lock()
	defer c.mu.Unlock()
	slots := c.wins[key]
	i := 0
	for _, t := range slots {
		if t.After(cutoff) {
			slots[i] = t
			i++
		}
	}
	slots = slots[:i]
	if len(slots) >= c.limit {
		c.wins[key] = slots
		return false
	}
	slots = append(slots, now)
	c.wins[key] = slots
	return true
}

func rateLimitKey(r *http.Request) string {
	ip := clientIP(r)
	if ip == nil {
		return r.RemoteAddr
	}
	return ip.String()
}

func writeRateLimited(w http.ResponseWriter) {
	http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
}

// wrapRPCLimits applies per-IP request and failed-auth limits before the handler runs.
func wrapRPCLimits(limits RPCLimits, auth *RPCAuth, next http.Handler) http.Handler {
	reqLim := newIPCounter(limits.RequestsPerMinute)
	authEnabled := auth != nil && auth.enabled()
	failLim := newIPCounter(limits.authFailCap(authEnabled))
	if reqLim == nil && failLim == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rateLimitKey(r)
		if reqLim != nil && !reqLim.allow(ip) {
			writeRateLimited(w)
			return
		}
		if authEnabled && failLim != nil {
			if !rpcAuthOK(auth, r) {
				RecordRPCAuthFailure()
				if !failLim.allow(ip) {
					writeRateLimited(w)
					return
				}
				writeWWWAuthenticate(w)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
