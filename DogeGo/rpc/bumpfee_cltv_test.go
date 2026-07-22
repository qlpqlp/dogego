// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"

	"dogego/secp256k1"

	"dogego/chain"
	"dogego/mempool"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func TestBumpFeeWalletCLTV(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k, _ := secp256k1.NewPrivateKey()
	wif, _ := chain.EncodeWIF(k.Serialize(), p.PrivKeyWIFVersion, true)
	addr, err := addressFromWIF("test", wif)
	if err != nil {
		t.Fatal(err)
	}
	desc := "sh(cltv(100)pkh(" + addr + "))"
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	paths := importDescTestPaths(w, p)
	paths.Utxo = store.NewUtxoCache()
	req, _ := json.Marshal(map[string]interface{}{"desc": desc, "keys": []string{wif}})
	_, code, msg := execImportDescriptors("test", paths, nil, nil, []json.RawMessage{json.RawMessage("[" + string(req) + "]")})
	if code != 0 {
		t.Fatalf("importdescriptors: %s", msg)
	}
	p2sh := w.WatchScripts()[0]
	changePK := p2sh
	fund := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2e8, PkScript: p2sh}},
	}
	_ = paths.Utxo.ApplyBlock(&wire.ParsedBlock{Txs: []*wire.Tx{fund}}, 0)
	prevHash := fund.TxHash()
	payTo, _ := chain.RandomP2PKHAddress(p)
	_, h160, _ := chain.Base58CheckDecode(payTo)
	outScript := chain.P2PKHScriptFromPubKeyHash(h160)
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout: []wire.TxOut{
			{Value: 5e7, PkScript: outScript},
			{Value: 12e7, PkScript: changePK},
		},
	}
	spendRaw, err := spend.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	pool := mempool.New(100)
	if err := pool.Add(spendRaw); err != nil {
		t.Fatal(err)
	}
	spendID := txidToRPC(spend.TxHash())
	paths.WalletWIFs = func() []string { wifs, _ := w.AllWIFs(p.PrivKeyWIFVersion); return wifs }
	j := &memJournal{tip: 2_000_000, count: 2_000_001, hdrs: [][]byte{make([]byte, 80)}}
	res, code2, msg2 := execBumpFee("test", pool, nil, nil, j, paths, []json.RawMessage{
		json.RawMessage(`"` + spendID + `"`),
		json.RawMessage(`{}`),
	}, nil, chain.RebootTestnet)
	if code2 != 0 {
		t.Fatalf("bumpfee: %d %s", code2, msg2)
	}
	newID, _ := res.(map[string]interface{})["txid"].(string)
	newRaw, err := pool.GetRawByTxID(newID)
	if err != nil || len(newRaw) == 0 {
		t.Fatalf("replacement not in pool: %v", err)
	}
	tx, err := wire.DeserializeTx(newRaw)
	if err != nil {
		t.Fatal(err)
	}
	if tx.LockTime < 100 {
		t.Fatalf("bumped tx LockTime=%d want >= 100", tx.LockTime)
	}
	_ = hex.EncodeToString(newRaw)
}
