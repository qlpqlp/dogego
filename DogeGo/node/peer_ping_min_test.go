// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"
	"time"
)

func TestPeerMinPing(t *testing.T) {
	var tr peerPingTracker
	var n1, n2 [8]byte
	copy(n1[:], []byte{1, 1, 1, 1, 1, 1, 1, 1})
	copy(n2[:], []byte{2, 2, 2, 2, 2, 2, 2, 2})
	tr.mu.Lock()
	tr.pending = n1
	tr.hasPending = true
	tr.sentAt = time.Now().Add(-80 * time.Millisecond)
	tr.mu.Unlock()
	tr.notePong(n1[:])
	if tr.minPingSeconds() <= 0 {
		t.Fatal("minping after first pong")
	}
	tr.mu.Lock()
	tr.pending = n2
	tr.hasPending = true
	tr.sentAt = time.Now().Add(-20 * time.Millisecond)
	tr.mu.Unlock()
	tr.notePong(n2[:])
	if tr.minPingSeconds() > tr.pingTimeSeconds() && tr.pingTimeSeconds() > 0 {
		t.Fatalf("minping %f should be <= last pingtime %f", tr.minPingSeconds(), tr.pingTimeSeconds())
	}
}
