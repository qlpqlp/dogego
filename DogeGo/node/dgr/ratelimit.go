// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import (
	"sync"
	"time"
)

type tokenBucket struct {
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
	mu     sync.Mutex
}

func newTokenBucket(rate, burst float64) *tokenBucket {
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = rate * 2
	}
	return &tokenBucket{rate: rate, burst: burst, tokens: burst, last: time.Now()}
}

func (b *tokenBucket) allow() bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now
	b.tokens += elapsed * b.rate
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

type registerLimiter struct {
	mu sync.Mutex
	by map[string]*registerWindow
	max int
}

type registerWindow struct {
	count int
	reset time.Time
}

func newRegisterLimiter(maxPerMin int) *registerLimiter {
	if maxPerMin <= 0 {
		maxPerMin = 10
	}
	return &registerLimiter{by: make(map[string]*registerWindow), max: maxPerMin}
}

func (l *registerLimiter) allow(key string) bool {
	if l == nil || key == "" {
		return true
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.by[key]
	if !ok || now.After(w.reset) {
		l.by[key] = &registerWindow{count: 1, reset: now.Add(time.Minute)}
		return true
	}
	if w.count >= l.max {
		return false
	}
	w.count++
	return true
}
