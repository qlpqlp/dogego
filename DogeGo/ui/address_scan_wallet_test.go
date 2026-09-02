// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"

	"dogego/wallet"
)

func TestScanAddressWalletFast(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	addWalletFastUtxo(cfg.UtxoCache(), 80, 0, 5_000_000_000, 400, spk)
	cfg.ActiveWallet().SeedScannedTx([]wallet.ScannedTx{{
		TxID: "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		Category: "receive", Address: cfg.ActiveWallet().DefaultAddress(), AmountKoinu: 5_000_000_000,
		Vout: 0, BlockHeight: 400,
	}})
	out, ok := ScanAddressWalletFast(cfg, cfg.ActiveWallet().DefaultAddress(), 0x71, 0xc4, 0, 40, 0, 40)
	if !ok || out == nil {
		t.Fatal("expected wallet fast path")
	}
	if out["wallet_fast"] != true {
		t.Fatalf("wallet_fast=%v", out["wallet_fast"])
	}
	if n, _ := out["matching_output_count"].(int); n < 1 {
		t.Fatalf("matching_output_count=%v", out["matching_output_count"])
	}
}

func TestAddressPkScriptSet(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	addr := cfg.ActiveWallet().DefaultAddress()
	set := addressPkScriptSet(addr, 0x71, 0xc4)
	if len(set) != 1 {
		t.Fatalf("set len=%d addr=%s", len(set), addr)
	}
}
