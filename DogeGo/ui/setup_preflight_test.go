// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"context"
	"testing"
)

func TestPortMatchesSetupOrigin(t *testing.T) {
	if !portMatchesSetupOrigin(2013, "http://127.0.0.1:2013") {
		t.Fatal("expected 2013 to match setup origin")
	}
	if portMatchesSetupOrigin(2014, "http://127.0.0.1:2013") {
		t.Fatal("expected port mismatch")
	}
}

func TestDogegoDashboardReachableLive(t *testing.T) {
	if !dogegoDashboardReachable("127.0.0.1", 2013) {
		t.Skip("no live dashboard on 127.0.0.1:2013")
	}
}

func TestBuildSetupPreflightOwnPort(t *testing.T) {
	resp := buildSetupPreflight(context.Background(), setupPreflightRequest{
		Network:         "testnet",
		P2PConnectivity: "both",
		Firewall:        "auto",
		WebUI:           "127.0.0.1:2013",
		SetupOrigin:     "http://127.0.0.1:2013",
	})
	for _, c := range resp.Checks {
		if c.ID == "webui" && c.Status != "ok" {
			t.Fatalf("webui check = %+v", c)
		}
	}
}

func TestBuildSetupPreflight(t *testing.T) {
	resp := buildSetupPreflight(context.Background(), setupPreflightRequest{
		Network:         "testnet",
		NodeMode:        "full",
		P2PConnectivity: "cgnat",
		Firewall:        "auto",
		UPnP:            "disable",
		WebUI:           "127.0.0.1:2013",
		RPC:             "127.0.0.1:22555",
	})
	if len(resp.Checks) < 3 {
		t.Fatalf("expected at least 3 checks, got %d", len(resp.Checks))
	}
	foundCGNAT := false
	for _, c := range resp.Checks {
		if c.ID == "cgnat" {
			foundCGNAT = true
			if c.Status != "ok" {
				t.Fatalf("cgnat status = %q", c.Status)
			}
		}
	}
	if !foundCGNAT {
		t.Fatal("missing cgnat check")
	}
}

func TestP2PPortForSetupNetwork(t *testing.T) {
	if p := p2pPortForSetupNetwork("mainnet"); p != 22556 {
		t.Fatalf("mainnet port = %d", p)
	}
	if p := p2pPortForSetupNetwork("testnet"); p != 44556 {
		t.Fatalf("testnet port = %d", p)
	}
}
