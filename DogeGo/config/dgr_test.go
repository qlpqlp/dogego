// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import "testing"

func TestDogeGoRelayCGNATDefaults(t *testing.T) {
	c := DogeGoRelayCGNAT{}
	if c.EffectiveListen() != ":24433" {
		t.Fatalf("listen %q", c.EffectiveListen())
	}
	if c.EffectiveRelayPort() != 24433 {
		t.Fatal(c.EffectiveRelayPort())
	}
	if c.EffectiveMaxClients() != 256 {
		t.Fatal(c.EffectiveMaxClients())
	}
	if c.EffectiveMaxRelayConns() != 3 {
		t.Fatal(c.EffectiveMaxRelayConns())
	}
	if c.EffectiveMaxSessionFramesPerSec() != 60 {
		t.Fatal(c.EffectiveMaxSessionFramesPerSec())
	}
	if c.EffectiveMaxP2PProxyPerSec() != 20 {
		t.Fatal(c.EffectiveMaxP2PProxyPerSec())
	}
	if c.EffectiveMaxRegisterPerMin() != 10 {
		t.Fatal(c.EffectiveMaxRegisterPerMin())
	}
}

func TestApplyWizardDGRDefaults(t *testing.T) {
	var f File
	f.P2PConnectivity = "both"
	ApplyWizardDGRDefaults(&f, false)
	if !f.DogeGoRelayCGNAT.Enabled || !f.DogeGoRelayCGNAT.InboundRelay || f.DogeGoRelayCGNAT.OutboundRelay {
		t.Fatalf("public both: %+v", f.DogeGoRelayCGNAT)
	}
	ApplyWizardDGRDefaults(&f, true)
	if !f.DogeGoRelayCGNAT.Enabled || f.DogeGoRelayCGNAT.InboundRelay || !f.DogeGoRelayCGNAT.OutboundRelay {
		t.Fatalf("cgnat both: %+v", f.DogeGoRelayCGNAT)
	}
	f = File{P2PConnectivity: "cgnat"}
	ApplyWizardDGRDefaults(&f, false)
	if !f.DogeGoRelayCGNAT.OutboundRelay || f.DogeGoRelayCGNAT.InboundRelay {
		t.Fatalf("cgnat mode: %+v", f.DogeGoRelayCGNAT)
	}
}

func TestEnsureDGRForP2P(t *testing.T) {
	var f File
	f.P2PConnectivity = "cgnat"
	EnsureDGRForP2P(&f)
	if !f.DogeGoRelayCGNAT.Enabled || !f.DogeGoRelayCGNAT.OutboundRelay || f.DogeGoRelayCGNAT.InboundRelay {
		t.Fatalf("cgnat auto: %+v", f.DogeGoRelayCGNAT)
	}
	f = File{P2PConnectivity: "both"}
	EnsureDGRForP2P(&f)
	if !f.DogeGoRelayCGNAT.Enabled || !f.DogeGoRelayCGNAT.InboundRelay {
		t.Fatalf("both auto: %+v", f.DogeGoRelayCGNAT)
	}
}

func TestDogeGoRelayCGNATRoles(t *testing.T) {
	inbound := DogeGoRelayCGNAT{Enabled: true, InboundRelay: true}
	if !inbound.RoleInbound() || !inbound.AdvertiseServiceBit() {
		t.Fatal("inbound")
	}
	client := DogeGoRelayCGNAT{Enabled: true}
	if !client.RoleOutbound("cgnat") {
		t.Fatal("auto outbound cgnat")
	}
	if client.RoleOutbound("classic") {
		t.Fatal("classic should not auto outbound")
	}
	explicit := DogeGoRelayCGNAT{Enabled: true, OutboundRelay: true}
	if !explicit.RoleOutbound("classic") {
		t.Fatal("explicit outbound")
	}
}

func TestValidateNormalizesDGREnabledFromRoles(t *testing.T) {
	f := File{
		DataDir: t.TempDir(),
		Network: "testnet",
		DogeGoRelayCGNAT: DogeGoRelayCGNAT{
			OutboundRelay: true,
			RelaySeeds:    []string{"203.0.113.1:24433"},
		},
	}
	if err := ValidateAndNormalize(&f); err != nil {
		t.Fatal(err)
	}
	if !f.DogeGoRelayCGNAT.Enabled {
		t.Fatal("expected enabled when outbound_relay set")
	}
}
