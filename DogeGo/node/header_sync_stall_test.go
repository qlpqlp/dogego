// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strings"
	"testing"
	"time"
)

func TestHeaderSyncStallLimit_earlyIBD(t *testing.T) {
	d := headerSyncStallLimit(4000, 6_221_339, true, 0, 0)
	if d != headerSyncStallEarlyIBD {
		t.Fatalf("want %v got %v", headerSyncStallEarlyIBD, d)
	}
}

func TestHeaderSyncStallLimit_earlyIBDAncientTipTimeNotMultiHour(t *testing.T) {
	now := int64(1_700_000_000)
	tipTime := now - 400*24*3600 // header tip years behind wall clock during early IBD
	// Peer not far ahead of local tip: must not apply Core's multi-hour historical-headers timeout.
	d := headerSyncStallLimit(4000, 4000, false, tipTime, now)
	if d != headerSyncStallDefault {
		t.Fatalf("early IBD must not use Core multi-hour stall window, got %v want %v", d, headerSyncStallDefault)
	}
}

func TestHeaderSyncStallLimit_past1MStillShortUntilAssumeValid(t *testing.T) {
	now := int64(1_700_000_000)
	tipTime := now - 400*24*3600
	d := headerSyncStallLimit(1_055_840, 6_300_000, false, tipTime, now)
	if d != headerSyncStallEarlyIBD {
		t.Fatalf("past 1M but before assumevalid with peer far ahead: want %v got %v", headerSyncStallEarlyIBD, d)
	}
	d2 := headerSyncStallLimit(1_055_840, 1_055_840, false, tipTime, now)
	if d2 != headerSyncStallDefault {
		t.Fatalf("past 1M ancient tip without far peer: want %v got %v", headerSyncStallDefault, d2)
	}
}

func TestHeaderSyncStallLimit_recentTipMayUseCoreWindow(t *testing.T) {
	now := int64(1_700_000_000)
	tipTime := now - 48*3600
	d := headerSyncStallLimit(5_100_000, 5_100_000, false, tipTime, now)
	if d < 15*time.Minute {
		t.Fatalf("recent tip past assumevalid may use Core window, got %v", d)
	}
}

func TestRecoverableHeaderPeerErr_stall(t *testing.T) {
	err := errString("header sync stall: no headers for 30s at tip 4000 (peer height 6221339)")
	if !recoverableHeaderPeerErr(err) {
		t.Fatal("want recoverable stall")
	}
}

func TestRecoverableHeaderPeerErr_yieldBackground(t *testing.T) {
	err := errString("header sync yielded at height 4000 (peer 6221339): background catch-up required")
	if !recoverableHeaderPeerErr(err) {
		t.Fatal("want recoverable yield")
	}
	if !isHeaderSyncYieldForBackground(err) {
		t.Fatal("want yield detector")
	}
	if !shouldAutoRecoverHeaderSync(err) {
		t.Fatal("want auto-recover")
	}
	if !strings.Contains(err.Error(), "background catch-up required") {
		t.Fatal("message")
	}
}
