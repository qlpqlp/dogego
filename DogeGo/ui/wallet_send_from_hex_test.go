// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/hex"
	"testing"

	"dogego/consensus"
	"dogego/mempool"
	"dogego/wallet"
	"dogego/wire"
)

func TestWalletSupplementMissingSendsPQ(t *testing.T) {
	cfg, addr, spk := testWalletFastSetup(t)
	u := cfg.UtxoCache()
	addWalletFastUtxo(u, 9, 0, 10_000_000_000, 40, spk)
	var prevHash [32]byte
	prevHash[0] = 9
	pqScript := make([]byte, 38)
	pqScript[0] = 0x6a
	pqScript[1] = 0x24
	copy(pqScript[2:6], []byte(consensus.PQTagFalcon))
	for i := 6; i < 38; i++ {
		pqScript[i] = byte(i)
	}
	tx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: prevHash,
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{
			{Value: 5_500_000_000, PkScript: spk},
			{Value: 0, PkScript: pqScript},
			{Value: 4_499_000_000, PkScript: spk},
		},
	}
	raw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	txid := mempool.TxIDDisplayHex(tx.TxHash())
	hx := hex.EncodeToString(raw)
	_ = cfg.ActiveWallet().RememberTxHex(txid, hx)
	cfg.ActiveWallet().SeedScannedTx([]wallet.ScannedTx{{
		TxID: txid, Category: "receive", Address: addr,
		AmountKoinu: 5_500_000_000, Vout: 0, BlockHeight: 50,
	}})
	entries := walletSupplementMissingSends(cfg, nil, cfg.ActiveWallet().ListScannedTx(), 50, "quantum")
	if len(entries) != 1 {
		t.Fatalf("entries=%d want 1", len(entries))
	}
	if entries[0]["tx_kind"] != "sent_pq" {
		t.Fatalf("tx_kind=%v", entries[0]["tx_kind"])
	}
	if amt, ok := entries[0]["amount"].(float64); !ok || amt >= 0 {
		t.Fatalf("amount=%v want negative send", entries[0]["amount"])
	}
}

func TestWalletSupplementMissingSendsSentFilter(t *testing.T) {
	cfg, addr, spk := testWalletFastSetup(t)
	u := cfg.UtxoCache()
	addWalletFastUtxo(u, 8, 0, 10_000_000_000, 40, spk)
	var prevHash [32]byte
	prevHash[0] = 8
	tx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: prevHash,
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{
			{Value: 5_500_000_000, PkScript: spk},
			{Value: 4_499_000_000, PkScript: spk},
		},
	}
	raw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	txid := mempool.TxIDDisplayHex(tx.TxHash())
	_ = cfg.ActiveWallet().RememberTxHex(txid, hex.EncodeToString(raw))
	cfg.ActiveWallet().SeedScannedTx([]wallet.ScannedTx{{
		TxID: txid, Category: "receive", Address: addr,
		AmountKoinu: 5_500_000_000, Vout: 0, BlockHeight: 50,
	}})
	entries := walletSupplementMissingSends(cfg, nil, cfg.ActiveWallet().ListScannedTx(), 50, "sent")
	if len(entries) != 1 {
		t.Fatalf("entries=%d want 1", len(entries))
	}
}

func TestScannedSendShowsPaymentNotChange(t *testing.T) {
	cfg, addr, spk := testWalletFastSetup(t)
	u := cfg.UtxoCache()
	addWalletFastUtxo(u, 7, 0, 10_000_000_000, 40, spk)
	var prevHash [32]byte
	prevHash[0] = 7
	external := []byte("external-payee-script")
	tx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: prevHash,
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{
			{Value: 4_499_000_000, PkScript: spk},
			{Value: 5_500_000_000, PkScript: external},
		},
	}
	raw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	txid := mempool.TxIDDisplayHex(tx.TxHash())
	_ = cfg.ActiveWallet().RememberTxHex(txid, hex.EncodeToString(raw))
	st := wallet.ScannedTx{
		TxID: txid, Category: "send", Address: addr,
		AmountKoinu: -4_499_000_000, FeeKoinu: 1_000_000, BlockHeight: 100,
	}
	entry := scannedSendToUIEntry(cfg, st, 100, "sent")
	if entry == nil {
		t.Fatal("nil entry")
	}
	amt, ok := entry["amount"].(float64)
	if !ok || amt != -55 {
		t.Fatalf("amount=%v want -55 (payment not change)", entry["amount"])
	}
}
