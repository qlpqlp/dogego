// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/chain"
	"dogego/secp256k1"
	"dogego/wire"
)

func TestBuildRelaxedLegacyBlockWithExtraTx(t *testing.T) {
	var prev [80]byte
	prev[0] = 1
	pubC := make([]byte, 33)
	pubC[0] = 0x02
	h160 := hash160(pubC)
	pkScript := append([]byte{0x21}, pubC...)
	pkScript = append(pkScript, 0xac)
	anchor := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: pkScript}},
	}
	display, payload, err := BuildRelaxedLegacyBlockPayload(prev[:], 1, chain.RebootTestnet, h160, []*wire.Tx{anchor})
	if err != nil {
		t.Fatal(err)
	}
	if display == "" || len(payload) < 81 {
		t.Fatalf("payload len=%d display=%q", len(payload), display)
	}
	if payload[80] != 2 {
		t.Fatalf("tx count=%d want 2 (coinbase + anchor)", payload[80])
	}
}

func TestAttachSubmitBlockPrepP2PKNonStandard(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x5a
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := standardP2PKHScript(h160[:])
	fund := WalletFundingUTXO{
		PrevHash: [32]byte{0xfe, 0xed},
		PrevIdx:  0,
		Value:    100_000_000,
		PkScript: pkScript,
	}
	probe, err := BuildWalletAnchoredStatefulProbe("p2pk_non_standard_input", priv, pubC, fund, 0)
	if err != nil {
		t.Fatal(err)
	}
	var prev [80]byte
	prev[0] = 1
	if err := AttachSubmitBlockPrep(&probe, prev[:], 1, chain.RebootTestnet, h160); err != nil {
		t.Fatal(err)
	}
	if probe.PrepSubmitBlockHex == "" {
		t.Fatal("expected prep_submit_block_hex")
	}
}
