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

func testPQSendTxHex(t *testing.T) (string, string) {
	t.Helper()
	script := make([]byte, 38)
	script[0] = 0x6a
	script[1] = 0x24
	copy(script[2:6], []byte(consensus.PQTagFalcon))
	for i := 6; i < 38; i++ {
		script[i] = byte(i)
	}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout: []wire.TxOut{
			{Value: 1, PkScript: []byte{0x51}},
			{Value: 0, PkScript: script},
		},
	}
	raw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return mempool.TxIDDisplayHex(tx.TxHash()), hex.EncodeToString(raw)
}

func TestScannedSendUIEntryHexBlockFallback(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	txid := "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234"
	cfg.Wallet.SeedScannedTx([]wallet.ScannedTx{{
		TxID: txid, Category: "send", Address: "DSendAddr", AmountKoinu: -1_000_000_000,
		BlockHeight: 50,
	}})
	// Block fallback needs a stored raw block with matching tx; setup has none here.
	if _, ok := cfg.Wallet.LookupTxHex(txid); ok {
		t.Fatal("unexpected hex in wallet.db before lookup")
	}
	entry := scannedSendToUIEntry(cfg, wallet.ScannedTx{
		TxID: txid, Category: "send", Address: "DSendAddr", AmountKoinu: -1_000_000_000, BlockHeight: 50,
	}, 50)
	if entry["hex"] != nil {
		t.Fatalf("hex %#v without block payload", entry["hex"])
	}
}

func TestWalletTxHexForUIMempoolPQEnrich(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	txid, hx := testPQSendTxHex(t)
	pool := mempool.New(10)
	raw, _ := hex.DecodeString(hx)
	if err := pool.Add(raw); err != nil {
		t.Fatal(err)
	}
	cfg.Pool = pool
	got := walletTxHexForUI(cfg, txid, -1)
	if got != hx {
		t.Fatalf("hex %q want %q", got, hx)
	}
	entry := scannedSendToUIEntry(cfg, wallet.ScannedTx{
		TxID: txid, Category: "send", Address: "DSendAddr", AmountKoinu: -5_500_000_000, BlockHeight: -1,
	}, 100)
	if entry["tx_kind"] != "sent_pq" {
		t.Fatalf("tx_kind %#v want sent_pq", entry["tx_kind"])
	}
	if entry["pq_tag"] != consensus.PQTagFalcon {
		t.Fatalf("pq_tag %#v want %s", entry["pq_tag"], consensus.PQTagFalcon)
	}
	if entry["confirmations"].(int64) != 0 {
		t.Fatalf("confirmations %#v want 0 for mempool send", entry["confirmations"])
	}
}
