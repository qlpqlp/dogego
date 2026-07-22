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

func TestBuildWalletPrevTxsIncludesRedeemScript(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := wallet.LoadOrCreate(t.TempDir()+"/wallet.json", p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	k, _ := secp256k1.NewPrivateKey()
	pub := hex.EncodeToString(k.PubKey().SerializeCompressed())
	nReq, _ := json.Marshal(1)
	keys, _ := json.Marshal([]string{pub})
	msRes, _, _ := execCreateMultisig("test", []json.RawMessage{nReq, keys})
	redeem, _ := hex.DecodeString(msRes["redeemScript"].(string))
	h := scriptHash160(redeem)
	p2sh := chain.P2SHScriptFromScriptHash(h)
	_ = w.AddWatchScript(p2sh)
	_ = w.SetWatchRedeem(p2sh, redeem)

	utxo := store.NewUtxoCache()
	fund := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1e9, PkScript: p2sh}},
	}
	_ = utxo.ApplyBlock(&wire.ParsedBlock{Txs: []*wire.Tx{fund}}, 0)
	prevHash := fund.TxHash()

	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5e8, PkScript: []byte{0x51}}},
	}
	paths := &DataPaths{
		Utxo:                    utxo,
		WalletWatchRedeemScript: func(spk []byte) []byte { return w.WatchRedeemScript(spk) },
	}
	prevs, err := buildWalletPrevTxs(spend, paths)
	if err != nil || len(prevs) != 1 {
		t.Fatalf("prevs len=%d err=%v", len(prevs), err)
	}
	var ent map[string]interface{}
	if err := json.Unmarshal(prevs[0], &ent); err != nil {
		t.Fatal(err)
	}
	if ent["redeemScript"] != hex.EncodeToString(redeem) {
		t.Fatalf("redeemScript %v", ent["redeemScript"])
	}
}
