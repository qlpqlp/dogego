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
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

// TestSignRawTransactionWithWalletP2SHMultisig signs a spend from an imported sh(multi) watch UTXO using cosigner keys in the wallet.
func TestSignRawTransactionWithWalletP2SHMultisig(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	h1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	h2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	nJ, _ := json.Marshal(1)
	keysJ, _ := json.Marshal([]string{h1, h2})
	msRes, code, msg := execCreateMultisig("test", []json.RawMessage{nJ, keysJ})
	if code != 0 {
		t.Fatalf("createmultisig: %s", msg)
	}
	redeem, _ := hex.DecodeString(msRes["redeemScript"].(string))
	h := scriptHash160(redeem)
	p2sh := chain.P2SHScriptFromScriptHash(h)

	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	_ = w.AddWatchScript(p2sh)
	_ = w.SetWatchRedeem(p2sh, redeem)
	wif1, _ := chain.EncodeWIF(k1.Serialize(), p.PrivKeyWIFVersion, true)
	if err := w.ImportPrivKey(wif1, p.PrivKeyWIFVersion, p.PubkeyHashAddrID); err != nil {
		t.Fatal(err)
	}

	utxo := store.NewUtxoCache()
	fund := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1e9, PkScript: p2sh}},
	}
	if err := utxo.ApplyBlock(&wire.ParsedBlock{Txs: []*wire.Tx{fund}}, 0); err != nil {
		t.Fatal(err)
	}
	prevHash := fund.TxHash()
	payTo, _ := chain.RandomP2PKHAddress(p)
	_, h160, _ := chain.Base58CheckDecode(payTo)
	pkScript := chain.P2PKHScriptFromPubKeyHash(h160)

	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5e8, PkScript: pkScript}},
	}
	spendHex, err := spend.Serialize()
	if err != nil {
		t.Fatal(err)
	}

	paths := walletTestPaths(w, p)
	paths.Utxo = utxo
	paths.WalletWatchScripts = func() [][]byte { return w.WatchScripts() }
	paths.WalletImportWatch = func(script []byte) error { return w.AddWatchScript(script) }

	res, code2, msg2 := execSignRawTransactionWithWallet("test", paths, []json.RawMessage{
		json.RawMessage(`"` + hex.EncodeToString(spendHex) + `"`),
	})
	if code2 != 0 {
		t.Fatalf("signrawtransactionwithwallet: %d %s", code2, msg2)
	}
	if !res["complete"].(bool) {
		t.Fatalf("complete=false errors=%v", res["errors"])
	}
}
