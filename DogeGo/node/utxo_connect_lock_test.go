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

func TestUtxoConnectInFlightIdle(t *testing.T) {
	if UtxoConnectInFlight() {
		t.Fatal("expected idle when connect mutex is free")
	}
}

func TestUtxoConnectInFlightWhileHeld(t *testing.T) {
	ready := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = withUtxoConnectLock(func() error {
			close(ready)
			<-release
			return nil
		})
	}()
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for connect lock")
	}
	if !UtxoConnectInFlight() {
		t.Fatal("expected in flight while connect mutex held")
	}
	close(release)
	time.Sleep(20 * time.Millisecond)
	if UtxoConnectInFlight() {
		t.Fatal("expected idle after connect mutex released")
	}
}
