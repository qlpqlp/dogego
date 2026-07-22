// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import "testing"

func TestMetricsSnapshotHealth(t *testing.T) {
	m := newMetrics()
	m.Enabled = true
	m.InboundRole = true
	m.ListenerOK = true
	m.ListenBound = "0.0.0.0:24433"
	m.RelayPort = 24433
	m.SetAdvertiseHost("203.0.113.5:22556", 24433)
	snap := m.Snapshot()
	if snap["health"] != "ok" {
		t.Fatalf("health=%v", snap["health"])
	}
	if snap["listen_bound"] != "0.0.0.0:24433" {
		t.Fatalf("bound=%v", snap["listen_bound"])
	}
	if snap["advertise_addr"] != "203.0.113.5:24433" {
		t.Fatalf("advertise=%v", snap["advertise_addr"])
	}
}

func TestTrimAdvertiseHost(t *testing.T) {
	if trimAdvertiseHost("203.0.113.5:22556") != "203.0.113.5" {
		t.Fatal("ipv4 host:port")
	}
	if trimAdvertiseHost("[2001:db8::1]:22556") != "[2001:db8::1]" {
		t.Fatal("ipv6")
	}
}
