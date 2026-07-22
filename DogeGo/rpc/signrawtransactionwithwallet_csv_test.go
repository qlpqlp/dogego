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
	"dogego/consensus"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func TestSignRawTransactionWithWalletImportDescriptorsShCSV(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	h1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	h2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	desc := "sh(csv(3)multi(1," + h1 + "," + h2 + "))"
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	wif1, _ := chain.EncodeWIF(k1.Serialize(), p.PrivKeyWIFVersion, true)
	paths := importDescTestPaths(w, p)
	paths.Utxo = store.NewUtxoCache()
	req, _ := json.Marshal(map[string]interface{}{"desc": desc, "keys": []string{wif1}})
	_, code, msg := execImportDescriptors("test", paths, nil, nil, []json.RawMessage{json.RawMessage("[" + string(req) + "]")})
	if code != 0 {
		t.Fatalf("importdescriptors: %s", msg)
	}
	p2sh := w.WatchScripts()[0]
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
		Version: 2,
		Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 1e8, PkScript: outScript}},
	}
	spendHex, err := spend.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	res, code2, msg2 := execSignRawTransactionWithWallet("test", paths, []json.RawMessage{
		json.RawMessage(`"` + hex.EncodeToString(spendHex) + `"`),
	})
	if code2 != 0 {
		t.Fatalf("signrawtransactionwithwallet: %d %s", code2, msg2)
	}
	if !res["complete"].(bool) {
		t.Fatalf("complete=false errors=%v", res["errors"])
	}
	signed, _ := hex.DecodeString(res["hex"].(string))
	tx, err := wire.DeserializeTx(signed)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Version < 2 {
		t.Fatalf("version=%d", tx.Version)
	}
	if !consensus.CSVInputSequenceSatisfies(tx.Vin[0].Sequence, 3) {
		t.Fatalf("sequence=%x", tx.Vin[0].Sequence)
	}
}
