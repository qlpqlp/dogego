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

	"dogego/secp256k1"

	"dogego/chain"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func TestFundRawTransactionP2SHChangeOutput(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := wallet.LoadOrCreate(t.TempDir()+"/wallet.json", p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	k, err := secp256k1.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pub := hex.EncodeToString(k.PubKey().SerializeCompressed())
	nReq, _ := json.Marshal(1)
	keys, _ := json.Marshal([]string{pub})
	msRes, code, msg := execCreateMultisig("test", []json.RawMessage{nReq, keys})
	if code != 0 {
		t.Fatal(msg)
	}
	redeem, _ := hex.DecodeString(msRes["redeemScript"].(string))
	h := scriptHash160(redeem)
	p2sh := chain.P2SHScriptFromScriptHash(h)
	_ = w.AddWatchScript(p2sh)
	_ = w.SetWatchRedeem(p2sh, redeem)

	utxo := store.NewUtxoCache()
	pb := &wire.ParsedBlock{Txs: []*wire.Tx{{
		Version: 1,
		Vin:     []wire.TxIn{{Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2e9, PkScript: w.P2PKHScript()}},
	}}}
	_ = utxo.ApplyBlock(pb, 0)

	paths := &DataPaths{
		Utxo:                    utxo,
		WalletP2PKHScript:       func() []byte { return w.P2PKHScript() },
		WalletWatchScripts:      func() [][]byte { return w.WatchScripts() },
		WalletWatchRedeemScript: func(spk []byte) []byte { return w.WatchRedeemScript(spk) },
	}
	payTo, _ := chain.RandomP2PKHAddress(p)
	outJ, _ := json.Marshal(map[string]interface{}{payTo: 1.0})
	inpJ, _ := json.Marshal([]interface{}{})
	rawHex, _, _ := execCreateRawTransaction("test", []json.RawMessage{inpJ, outJ})
	optsJ, _ := json.Marshal(map[string]interface{}{"changeAddress": msRes["address"]})
	res, code2, msg2 := execFundRawTransaction("test", paths, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"` + rawHex.(string) + `"`),
		optsJ,
	})
	if code2 != 0 {
		t.Fatalf("fund: %d %q", code2, msg2)
	}
	txRaw, _ := hex.DecodeString(res.(map[string]interface{})["hex"].(string))
	tx, err := wire.DeserializeTx(txRaw)
	if err != nil {
		t.Fatal(err)
	}
	cp := res.(map[string]interface{})["changepos"]
	var changeIdx int
	switch v := cp.(type) {
	case float64:
		changeIdx = int(v)
	case int:
		changeIdx = v
	default:
		t.Fatalf("changepos %T", cp)
	}
	if !bytesEqual(tx.Vout[changeIdx].PkScript, p2sh) {
		t.Fatalf("change script %x want P2SH %x", tx.Vout[changeIdx].PkScript, p2sh)
	}
}

func bytesEqual(a, b []byte) bool {
	return len(a) == len(b) && string(a) == string(b)
}
