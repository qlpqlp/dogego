// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"dogego/chain"
	"dogego/mempool"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestWalletTxSpendsFromWallet(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr, _ := chain.RandomP2PKHAddress(p)
	pk := p2pkhScriptForTest(t, p, addr)
	utxo := store.NewUtxoCache()
	fund := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1e8, PkScript: pk}},
	}
	_ = utxo.ApplyBlock(&wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{fund}}, 0)
	var prevHash [32]byte
	fh := fund.TxHash()
	copy(prevHash[:], fh[:])
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 9e7, PkScript: []byte{0x51}}},
	}
	paths := &DataPaths{
		Utxo: utxo,
		WalletSpendScripts: func() [][]byte {
			return [][]byte{append([]byte(nil), pk...)}
		},
	}
	if !walletTxSpendsFromWallet(paths, spend) {
		t.Fatal("expected wallet spend")
	}
	if walletTxSpendsFromWallet(paths, fund) {
		t.Fatal("coinbase should not match")
	}
}

func TestExecSendRawTransactionRecordsWalletHex(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr, _ := chain.RandomP2PKHAddress(p)
	pk := p2pkhScriptForTest(t, p, addr)
	utxo := store.NewUtxoCache()
	fund := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1e8, PkScript: pk}},
	}
	_ = utxo.ApplyBlock(&wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{fund}}, 0)
	var prevHash [32]byte
	fh := fund.TxHash()
	copy(prevHash[:], fh[:])
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 9e7, PkScript: []byte{0x51}}},
	}
	raw, _ := spend.Serialize()
	hexStr := hex.EncodeToString(raw)
	txid := mempool.TxIDDisplayHex(spend.TxHash())
	var stored string
	paths := &DataPaths{
		Utxo: utxo,
		WalletSpendScripts: func() [][]byte {
			return [][]byte{append([]byte(nil), pk...)}
		},
		WalletRememberTxHex: func(id, h string) error {
			if id == txid {
				stored = h
			}
			return nil
		},
	}
	pool := mempool.New(10)
	param, _ := json.Marshal(hexStr)
	res, code, msg := execSendRawTransaction(pool, nil, nil, nil, paths, []json.RawMessage{param}, nil, true, chain.RebootTestnet)
	if code != 0 {
		t.Fatalf("sendraw code=%d msg=%s", code, msg)
	}
	if res.(string) != txid {
		t.Fatalf("txid %v", res)
	}
	if stored != hexStr {
		t.Fatalf("stored hex %q want %q", stored, hexStr)
	}
}
