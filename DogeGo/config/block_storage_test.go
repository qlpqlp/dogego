// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import (
	"testing"

	"dogego/store"
)

func TestSetupWizardSeed_recommendedStorage(t *testing.T) {
	f := SetupWizardSeed(File{NodeMode: "full", Network: "testnet"})
	if f.BlockStorageLayout != store.BlockLayoutBundled {
		t.Fatalf("layout %q", f.BlockStorageLayout)
	}
	if !f.BlockZstd {
		t.Fatal("want block_zstd")
	}
	if f.EffectiveTxIndexEmbedTx() {
		t.Fatal("want compact tx index (embed off)")
	}
}

func TestSetupWizardSeed_networkDefaults(t *testing.T) {
	f := SetupWizardSeed(File{NodeMode: "full", Network: "testnet"})
	if f.P2PConnectivity != "both" {
		t.Fatalf("p2p %q", f.P2PConnectivity)
	}
	if f.Firewall != "auto" || f.Upnp != "auto" {
		t.Fatalf("firewall=%q upnp=%q", f.Firewall, f.Upnp)
	}
	if !f.Mine {
		t.Fatal("testnet wizard should enable auto mining")
	}
	spv := SetupWizardSeed(File{NodeMode: "spv"})
	if spv.P2PConnectivity != "cgnat" {
		t.Fatalf("spv p2p %q", spv.P2PConnectivity)
	}
}

func TestSetupWizardSeed_spvSkipsStorage(t *testing.T) {
	f := SetupWizardSeed(File{NodeMode: "spv"})
	if f.BlockStorageLayout != "" {
		t.Fatalf("spv should not set layout, got %q", f.BlockStorageLayout)
	}
}

func TestSetupWizardSeed_securityDefaults(t *testing.T) {
	f := SetupWizardSeed(File{NodeMode: "full", Network: "testnet"})
	if !f.WebUITLSLocal {
		t.Fatal("want webui_tls_local default")
	}
	if !f.LocalTLSTrustCA {
		t.Fatal("want local_tls_trust_ca default")
	}
	if f.DataDir == "" {
		t.Fatal("want default datadir")
	}
}
