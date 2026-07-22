// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/chain"
	"dogego/mempool"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestWalletSendFeeKoinuSpendScripts(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr1, _ := chain.RandomP2PKHAddress(p)
	addr2, _ := chain.RandomP2PKHAddress(p)
	pk1 := p2pkhScriptForTest(t, p, addr1)
	pk2 := p2pkhScriptForTest(t, p, addr2)

	utxo := store.NewUtxoCache()
	fund := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000_000, PkScript: pk2}},
	}
	_ = utxo.ApplyBlock(&wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{fund}}, 0)

	var prevHash [32]byte
	fh := fund.TxHash()
	copy(prevHash[:], fh[:])
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: prevHash,
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{
			{Value: 8_000_000_000, PkScript: pk1},
			{Value: 1_990_000_000, PkScript: pk2},
		},
	}
	pool := mempool.New(10)
	raw, _ := spend.Serialize()
	_ = pool.Add(raw)
	txid := mempool.TxIDDisplayHex(spend.TxHash())

	paths := &DataPaths{
		Utxo: utxo,
		WalletSpendScripts: func() [][]byte {
			return [][]byte{append([]byte(nil), pk1...), append([]byte(nil), pk2...)}
		},
	}
	fee := walletSendFeeKoinu("testnet", paths, pool, nil, nil, nil, txid, -1)
	if fee != 10_000_000 {
		t.Fatalf("fee %d want 10000000 (10M koinu)", fee)
	}
}

func TestWalletSendFeeKoinuScanLookup(t *testing.T) {
	txid := repeatHex('c')
	paths := &DataPaths{
		WalletSendFeeLookup: func(id string) (int64, bool) {
			if id == txid {
				return 5_000_000, true
			}
			return 0, false
		},
	}
	fee := walletSendFeeKoinu("testnet", paths, nil, nil, nil, nil, txid, 100)
	if fee != 5_000_000 {
		t.Fatalf("lookup fee %d want 5000000", fee)
	}
}

func TestWalletSendFeeKoinuExternalChange(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr1, _ := chain.RandomP2PKHAddress(p)
	addr2, _ := chain.RandomP2PKHAddress(p)
	extAddr, _ := chain.RandomP2PKHAddress(p)
	pk1 := p2pkhScriptForTest(t, p, addr1)
	pk2 := p2pkhScriptForTest(t, p, addr2)
	extPk := p2pkhScriptForTest(t, p, extAddr)

	utxo := store.NewUtxoCache()
	fund := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000_000, PkScript: pk2}},
	}
	_ = utxo.ApplyBlock(&wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{fund}}, 0)

	var prevHash [32]byte
	fh := fund.TxHash()
	copy(prevHash[:], fh[:])
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: prevHash,
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{
			{Value: 8_000_000_000, PkScript: extPk},
			{Value: 1_990_000_000, PkScript: pk1},
		},
	}
	pool := mempool.New(10)
	raw, _ := spend.Serialize()
	_ = pool.Add(raw)
	txid := mempool.TxIDDisplayHex(spend.TxHash())

	paths := &DataPaths{
		Utxo: utxo,
		WalletSpendScripts: func() [][]byte {
			return [][]byte{append([]byte(nil), pk1...), append([]byte(nil), pk2...)}
		},
	}
	fee := walletSendFeeKoinu("testnet", paths, pool, nil, nil, nil, txid, -1)
	if fee != 10_000_000 {
		t.Fatalf("external send fee %d want 10000000 (0.1 DOGE)", fee)
	}
}

func TestWalletReceiveTxKindCompactHeuristic(t *testing.T) {
	ix := &store.TxIndex{EmbedTx: false}
	row := walletTxRow{
		category:      "receive",
		vout:          0,
		confirmations: 5,
		blockHeight:   100,
	}
	if got := walletReceiveTxKind("testnet", nil, nil, nil, ix, row); got != "mining_immature" {
		t.Fatalf("compact vout0 immature: got %q want mining_immature", got)
	}
	row.confirmations = 30
	if got := walletReceiveTxKind("testnet", nil, nil, nil, ix, row); got != "mining" {
		t.Fatalf("compact vout0 mature: got %q want mining", got)
	}
	row.vout = 1
	if got := walletReceiveTxKind("testnet", nil, nil, nil, ix, row); got != "received" {
		t.Fatalf("compact vout1 without index: got %q want received", got)
	}
}

func p2pkhScriptForTest(t *testing.T, p chain.Params, addr string) []byte {
	t.Helper()
	_, h160, err := chain.Base58CheckDecode(addr)
	if err != nil {
		t.Fatal(err)
	}
	pk := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	return append(pk, 0x88, 0xac)
}
