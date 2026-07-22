// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"bytes"
	"crypto/rand"
	"sync"
	"time"
)

const peerPingInterval = 2 * time.Minute

// peerPingTracker sends periodic outbound pings and measures RTT from matching pongs (Core getpeerinfo pingtime).
type peerPingTracker struct {
	mu         sync.Mutex
	lastSent   time.Time
	sentAt     time.Time
	pending    [8]byte
	hasPending bool
	rtt        time.Duration
	minRTT     time.Duration
}

// forcePing sends an outbound ping immediately (Core RPC ping / fPingQueued).
func (t *peerPingTracker) forcePing(mw *MsgWriter) {
	if t == nil || mw == nil {
		return
	}
	t.mu.Lock()
	t.lastSent = time.Time{}
	t.mu.Unlock()
	t.maybePing(mw)
}

func (t *peerPingTracker) maybePing(mw *MsgWriter) {
	if t == nil || mw == nil {
		return
	}
	t.mu.Lock()
	if !t.lastSent.IsZero() && time.Since(t.lastSent) < peerPingInterval {
		t.mu.Unlock()
		return
	}
	var nonce [8]byte
	_, _ = rand.Read(nonce[:])
	t.pending = nonce
	t.hasPending = true
	t.sentAt = time.Now()
	t.lastSent = t.sentAt
	t.mu.Unlock()
	_ = mw.Write("ping", nonce[:])
}

// notePong records RTT when payload echoes our last outbound ping nonce.
func (t *peerPingTracker) notePong(payload []byte) {
	if t == nil || len(payload) != 8 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.hasPending {
		return
	}
	var got [8]byte
	copy(got[:], payload)
	if got != t.pending {
		return
	}
	t.rtt = time.Since(t.sentAt)
	t.hasPending = false
	if t.rtt > 0 && (t.minRTT == 0 || t.rtt < t.minRTT) {
		t.minRTT = t.rtt
	}
}

// minPingSeconds returns Core getpeerinfo minping (minimum observed RTT), or 0 when unknown.
func (t *peerPingTracker) minPingSeconds() float64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.minRTT <= 0 {
		return 0
	}
	return t.minRTT.Seconds()
}

// pingWaitSeconds returns Core getpeerinfo pingwait while an outbound ping is unanswered.
func (t *peerPingTracker) pingWaitSeconds() float64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.hasPending || t.sentAt.IsZero() {
		return 0
	}
	return time.Since(t.sentAt).Seconds()
}

// pingTimeSeconds returns Core-style pingtime in seconds, or 0 when unknown.
func (t *peerPingTracker) pingTimeSeconds() float64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.rtt <= 0 {
		return 0
	}
	return t.rtt.Seconds()
}

// replyPing answers an inbound ping (payload echoed in pong).
func replyPing(mw *MsgWriter, payload []byte) error {
	if mw == nil {
		return nil
	}
	return mw.Write("pong", payload)
}

// pingNonceMatches reports whether pong payload matches an expected ping nonce.
func pingNonceMatches(payload, nonce []byte) bool {
	return len(payload) == 8 && len(nonce) == 8 && bytes.Equal(payload, nonce)
}
