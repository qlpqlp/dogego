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
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestFundRawTxLockUnspentsFalse(t *testing.T) {
	utxo := store.NewUtxoCache()
	spendPK := p2pkhScript(t, chain.RebootTestnet)
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000_000, PkScript: spendPK}},
	}
	pb := &wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}
	if err := utxo.ApplyBlock(pb, 0); err != nil {
		t.Fatal(err)
	}
	coinID := txidToRPC(coin.TxHash())
	paths := &DataPaths{
		Utxo: utxo,
		WalletP2PKHScript: func() []byte { return spendPK },
		WalletIsLockedOutpoint: func(txid string, vout uint32) bool {
			return txid == coinID && vout == 0
		},
	}
	empty := &wire.Tx{Version: 1, Vout: []wire.TxOut{{Value: 1_000_000_000, PkScript: spendPK}}}
	emptyRaw, _ := empty.Serialize()
	hexTx := hex.EncodeToString(emptyRaw)
	tp, _ := chain.ParamsFor(chain.RebootTestnet)
	changeAddr, _ := chain.RandomP2PKHAddress(tp)
	optsLock, _ := json.Marshal(map[string]interface{}{"changeAddress": changeAddr, "lockUnspents": true})
	_, code, _ := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"` + hexTx + `"`),
		optsLock,
	})
	if code != -6 {
		t.Fatalf("locked+lockUnspents: code=%d want -6", code)
	}
	optsUnlock, _ := json.Marshal(map[string]interface{}{"changeAddress": changeAddr, "lockUnspents": false})
	res, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"` + hexTx + `"`),
		optsUnlock,
	})
	if code != 0 {
		t.Fatalf("lockUnspents false: %d %s", code, msg)
	}
	if res == nil {
		t.Fatal("nil result")
	}
}

func TestParseFundRawTxIncludeWatchingDefault(t *testing.T) {
	o, code, msg := parseFundRawTxOptions(nil, nil)
	if code != 0 || msg != "" {
		t.Fatalf("%d %s", code, msg)
	}
	if o.includeWatching || !o.lockUnspents {
		t.Fatalf("defaults: watch=%v lock=%v", o.includeWatching, o.lockUnspents)
	}
}
