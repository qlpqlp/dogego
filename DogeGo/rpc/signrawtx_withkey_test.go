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
)

func TestExecSignRawTransactionWithKeyRequiresPrivkeys(t *testing.T) {
	_, code, msg := execSignRawTransactionWithKey("testnet", nil, []json.RawMessage{
		json.RawMessage(`"01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff00f0ffffff00"`),
		json.RawMessage(`[]`),
	})
	if code != -8 || msg == "" {
		t.Fatalf("want -8 privkeys required, got %d %q", code, msg)
	}
}

func TestExecSignRawTransactionWithKeyNoWalletMerge(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	sec := make([]byte, 32)
	sec[0] = 0x22
	wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	payTo, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	prevTxid := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	inp, _ := json.Marshal([]map[string]interface{}{{"txid": prevTxid, "vout": 0}})
	outObj := map[string]interface{}{payTo: 0.1}
	outJSON, _ := json.Marshal(outObj)
	rawHex, code, msg := execCreateRawTransaction("test", []json.RawMessage{inp, outJSON})
	if code != 0 {
		t.Fatalf("createraw: %d %s", code, msg)
	}
	pubC := mustPubCompressed(t, sec)
	h160 := pubkeyHash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)
	prevEntry := map[string]interface{}{
		"txid": prevTxid, "vout": 0, "scriptPubKey": hex.EncodeToString(pkScript),
	}
	prevArr, _ := json.Marshal([]map[string]interface{}{prevEntry})
	privArr, _ := json.Marshal([]string{wif})
	paths := &DataPaths{
		WalletAddress: func() string { return "DWalletShouldNotBeUsed1111111111111" },
	}
	res, code2, msg2 := execSignRawTransactionWithKey("test", paths, []json.RawMessage{
		json.RawMessage(`"` + rawHex.(string) + `"`),
		prevArr,
		privArr,
	})
	if code2 != 0 {
		t.Fatalf("withkey: %d %s", code2, msg2)
	}
	if !res["complete"].(bool) {
		t.Fatalf("complete=false errors=%v", res["errors"])
	}
}
