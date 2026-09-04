// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dogeos

import (
	"testing"
)

func TestChikyuChainIDHex(t *testing.T) {
	n, ok := FindNetwork(NetworkChikyuTestnet)
	if !ok {
		t.Fatal("missing chikyu")
	}
	if n.ChainID != 6281971 {
		t.Fatalf("chain id %d", n.ChainID)
	}
	if got := FormatChainIDHex(n.ChainID); got != "0x5fdaf3" {
		t.Fatalf("hex %s", got)
	}
	if !n.Available {
		t.Fatal("chikyu should be available")
	}
}

func TestMainnetPlaceholder(t *testing.T) {
	n, ok := FindNetwork(NetworkMainnetSoon)
	if !ok || n.Available {
		t.Fatal("mainnet should exist and be unavailable")
	}
}

func TestHelpers(t *testing.T) {
	n, _ := FindNetwork(NetworkChikyuTestnet)
	h := Helpers(n, n.RPCURL)
	if h["chain_id_hex"] != "0x5fdaf3" {
		t.Fatalf("%v", h["chain_id_hex"])
	}
	mm, ok := h["metamask_add"].(map[string]interface{})
	if !ok || mm["chainId"] != "0x5fdaf3" {
		t.Fatalf("metamask %+v", mm)
	}
}

func TestNormalizeAddress(t *testing.T) {
	if normalizeAddress("0xAbcDef0123456789AbcDef0123456789AbCdEf01") == "" {
		t.Fatal("valid addr rejected")
	}
	if normalizeAddress("not-an-address") != "" {
		t.Fatal("invalid accepted")
	}
}

func TestParseHex(t *testing.T) {
	if parseHexInt64("0x5fdaf3") != 6281971 {
		t.Fatal("parse chain")
	}
	if parseHexBig("0xff") != "255" {
		t.Fatal("parse big")
	}
}
