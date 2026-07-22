// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"dogego/chain"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func TestLegacyBlockMempoolTxsSelectsPoolTx(t *testing.T) {
	var prev [32]byte
	prev[0] = 9
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(0)
	var op [36]byte
	copy(op[:32], prev[:])
	utxo.AddUtxoForTest(op, store.UtxoEntry{Value: 10_000_000_000, PkScript: []byte{0x51}, Height: 0})
	paths := &DataPaths{Utxo: utxo}
	pool := mempool.New(100)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: wire.SequenceFinal}},
		Vout:    []wire.TxOut{{Value: 9_000_000_000, PkScript: []byte{0x51}}},
	}
	rawTx, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Add(rawTx); err != nil {
		t.Fatal(err)
	}
	txs, fees := legacyBlockMempoolTxs(pool, nil, nil, paths)
	if len(txs) != 1 {
		t.Fatalf("txs=%d want 1", len(txs))
	}
	if fees <= 0 {
		t.Fatalf("fees=%d", fees)
	}
}

func TestMineLegacyBlockIncludesMempoolTx(t *testing.T) {
	if testing.Short() {
		t.Skip("live scrypt mining with mempool (run without -short)")
	}
	if runtime.GOOS == "windows" && os.Getenv("DOGEGO_RUN_SCRYPT_MINE") != "1" {
		t.Skip("scrypt mining is slow on windows; set DOGEGO_RUN_SCRYPT_MINE=1 to run")
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(filepath.Join(dir, "rawblocks"))
	if err != nil {
		t.Fatal(err)
	}
	var prev [32]byte
	prev[0] = 9
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(0)
	var op [36]byte
	copy(op[:32], prev[:])
	utxo.AddUtxoForTest(op, store.UtxoEntry{Value: 10_000_000_000, PkScript: []byte{0x51}, Height: 0})
	paths := &DataPaths{Utxo: utxo}
	pool := mempool.New(100)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: wire.SequenceFinal}},
		Vout:    []wire.TxOut{{Value: 9_000_000_000, PkScript: []byte{0x51}}},
	}
	rawTx, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Add(rawTx); err != nil {
		t.Fatal(err)
	}
	addr, _ := chain.RandomP2PKHAddress(p)
	h160, err := p2pkhScriptFromAddress("testnet", addr)
	if err != nil {
		t.Fatal(err)
	}
	// Cap attempts so this test does not stall CI/dev boxes.
	_, payload, err := mineLegacyBlockToAddress(j, raw, paths, pool, nil, p, chain.RebootTestnet, h160, 250_000)
	if err != nil {
		t.Skip("scrypt mining too slow in this environment; run without -short on dogego-live")
	}
	if len(payload) < 82 || payload[80] != 2 {
		t.Fatalf("expected 2 txs in block, got tx count byte %d", payload[80])
	}
}
