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
	"dogego/config"
	"dogego/wallet"
)

func TestWalletAPIEnvelopeSignerCmdConfigured(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	out := walletAPIEnvelope(StartConfig{
		Wallet:        w,
		Network:       "testnet",
		EffectiveFile: config.File{SignerCmd: "python hwi.py --stdin"},
	})
	if configured, _ := out["signer_cmd_configured"].(bool); !configured {
		t.Fatalf("signer_cmd_configured=%v", out["signer_cmd_configured"])
	}
}
