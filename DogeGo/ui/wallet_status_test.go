// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/wallet"
)

func TestAttachWalletEncryptionStatus(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	summary := map[string]any{}
	attachWalletEncryptionStatus(summary, w)
	if enc, ok := summary["wallet_encrypted"].(bool); !ok || enc {
		t.Fatalf("plaintext wallet_encrypted=%#v", summary["wallet_encrypted"])
	}
	if unlocked, ok := summary["wallet_unlocked"].(bool); !ok || !unlocked {
		t.Fatalf("plaintext wallet_unlocked=%#v", summary["wallet_unlocked"])
	}
	if _, err := w.Encrypt("hunter2"); err != nil {
		t.Fatal(err)
	}
	summary = map[string]any{}
	attachWalletEncryptionStatus(summary, w)
	if enc, ok := summary["wallet_encrypted"].(bool); !ok || !enc {
		t.Fatalf("encrypted wallet_encrypted=%#v", summary["wallet_encrypted"])
	}
	if unlocked, ok := summary["wallet_unlocked"].(bool); !ok || unlocked {
		t.Fatalf("locked wallet_unlocked=%#v", summary["wallet_unlocked"])
	}
}

func TestWalletStatusJSONKeypoolFields(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	out := walletStatusJSON(w, "testnet")
	n, ok := out["keypool_size"].(int)
	if !ok || n <= 0 {
		t.Fatalf("keypool_size=%#v", out["keypool_size"])
	}
	if chg, ok := out["change_keypool_size"].(int); !ok || chg <= 0 {
		t.Fatalf("change_keypool_size=%#v", out["change_keypool_size"])
	}
	if hd, ok := out["hd_wallet"].(bool); !ok || !hd {
		t.Fatalf("hd_wallet=%#v", out["hd_wallet"])
	}
}

func TestMergeWalletInfoKeypoolDoesNotOverwriteDisk(t *testing.T) {
	out := map[string]any{
		"pool_core_indices_stored": 2,
		"hd_keypool_core_index":    map[string]int64{"0": 1},
	}
	mergeWalletInfoKeypool(out, map[string]interface{}{
		"pool_core_indices_stored": float64(99),
		"hd_keypool_core_index":    map[string]interface{}{"9": float64(9)},
	})
	if out["pool_core_indices_stored"].(int) != 2 {
		t.Fatalf("overwrote pool_core_indices_stored: %+v", out)
	}
}

func TestWalletAPIEnvelopeIncludesKeypoolFromDisk(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	out := walletAPIEnvelope(cfg)
	if out["keypool_size"] == nil {
		t.Fatalf("missing keypool_size: %+v", out)
	}
}
