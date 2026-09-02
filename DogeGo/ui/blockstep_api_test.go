// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dogego/consensus"
)

func TestBlockstepVoutNavPQCommitment(t *testing.T) {
	script := make([]byte, 38)
	script[0] = 0x6a
	script[1] = 0x24
	copy(script[2:6], []byte(consensus.PQTagFalcon))
	jm := map[string]interface{}{
		"vout": []interface{}{
			map[string]interface{}{
				"n":     float64(1),
				"value": float64(0),
				"scriptPubKey": map[string]interface{}{
					"hex": hex.EncodeToString(script),
					"asm": "OP_RETURN ...",
				},
			},
		},
	}
	outs := blockstepVoutNav(jm, 0x1e, 0x16)
	if len(outs) != 1 {
		t.Fatalf("outs=%d", len(outs))
	}
	if outs[0]["pq_tag"] != consensus.PQTagFalcon {
		t.Fatalf("pq_tag=%v", outs[0]["pq_tag"])
	}
	if outs[0]["output_kind"] != "pq_commitment" {
		t.Fatalf("output_kind=%v", outs[0]["output_kind"])
	}
	summary := blockstepTxPQSummary(outs)
	if summary == nil || summary["tag"] != consensus.PQTagFalcon {
		t.Fatalf("summary=%#v", summary)
	}
}

func TestBlockstepStatusIcon(t *testing.T) {
	if blockstepStatusIcon(true, true) == "" {
		t.Fatal("expected icon")
	}
	if blockstepStatusLabel(false, false) != "headers_only" {
		t.Fatalf("label = %q", blockstepStatusLabel(false, false))
	}
}

func TestBlockstepAvailability(t *testing.T) {
	block := map[string]any{"has_raw_block": true}
	av := blockstepAvailability(block, nil)
	if av["status"] != "partial" {
		t.Fatalf("status = %v", av["status"])
	}
}

func TestBlockstepNavigableHeight(t *testing.T) {
	if got := blockstepNavigableHeight(534000, 8061, 0, true); got != 8061 {
		t.Fatalf("stored bodies define navigable range: got %d want 8061", got)
	}
	if got := blockstepNavigableHeight(534000, -1, 500, true); got != 500 {
		t.Fatalf("chain active fallback: got %d want 500", got)
	}
	if got := blockstepNavigableHeight(100, -1, -1, false); got != 100 {
		t.Fatalf("headers only: got %d want 100", got)
	}
}

func TestBlockstepScriptDisplayP2PKH(t *testing.T) {
	// P2PKH script for a dummy hash160
	script := []byte{0x76, 0xa9, 0x14,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09,
		0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13,
		0x88, 0xac}
	addr, kind, _ := blockstepScriptDisplay(script, 0x1e, 0x16)
	if addr == "" || kind != "address" {
		t.Fatalf("addr=%q kind=%q", addr, kind)
	}
}

func TestBlockStepAddressWalletFastHTTP(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	addWalletFastUtxo(cfg.UtxoCache(), 80, 0, 5_000_000_000, 400, spk)
	addr := cfg.ActiveWallet().DefaultAddress()
	mux := http.NewServeMux()
	registerBlockStepRoutes(mux, cfg)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/blockstep/address?address=" + addr)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["wallet_fast"] != true {
		t.Fatalf("wallet_fast=%v", out["wallet_fast"])
	}
}
