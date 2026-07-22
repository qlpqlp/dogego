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

func TestFundRawTransactionP2SHWatchScript(t *testing.T) {
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
		t.Fatalf("multisig: %d %q", code, msg)
	}
	redeem, _ := hex.DecodeString(msRes["redeemScript"].(string))
	h := scriptHash160(redeem)
	p2sh := chain.P2SHScriptFromScriptHash(h)
	if err := w.AddWatchScript(p2sh); err != nil {
		t.Fatal(err)
	}

	utxo := store.NewUtxoCache()
	pb := &wire.ParsedBlock{
		Txs: []*wire.Tx{{
			Version: 1,
			Vin:     []wire.TxIn{{Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1e9, PkScript: p2sh}},
		}},
	}
	if err := utxo.ApplyBlock(pb, 0); err != nil {
		t.Fatal(err)
	}

	paths := &DataPaths{
		Utxo:               utxo,
		WalletWatchScripts: func() [][]byte { return w.WatchScripts() },
	}
	payTo, _ := chain.RandomP2PKHAddress(p)
	outJ, _ := json.Marshal(map[string]interface{}{payTo: 0.5})
	inpJ, _ := json.Marshal([]interface{}{})
	rawHex, code, msg := execCreateRawTransaction("test", []json.RawMessage{inpJ, outJ})
	if code != 0 {
		t.Fatalf("create: %d %q", code, msg)
	}
	optsJ, _ := json.Marshal(map[string]interface{}{
		"changeAddress":    w.Address(),
		"includeWatching":  true,
	})
	res, code2, msg2 := execFundRawTransaction("test", paths, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"` + rawHex.(string) + `"`),
		optsJ,
	})
	if code2 != 0 || msg2 != "" {
		t.Fatalf("fund: %d %q", code2, msg2)
	}
	m := res.(map[string]interface{})
	if m["hex"] == nil || m["hex"] == "" {
		t.Fatalf("result %#v", m)
	}
}
