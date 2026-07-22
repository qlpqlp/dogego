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
	"dogego/consensus"
)

func TestExecSignRawTransactionP2SHCSVP2PKH(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	sec := make([]byte, 32)
	sec[0] = 0x16
	wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	pubC := mustPubCompressed(t, sec)
	h160 := pubkeyHash160(pubC)
	relSeq := int64(3)
	redeem := consensus.BuildCSVP2PKHRedeemScript(relSeq, h160)
	rh := hash160P2SH(redeem)
	p2sh := append([]byte{0xa9, 0x14}, rh[:]...)
	p2sh = append(p2sh, 0x87)
	payTo, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}

	prevTxid := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	inp, _ := json.Marshal([]map[string]interface{}{{"txid": prevTxid, "vout": 0, "sequence": relSeq}})
	outObj := map[string]interface{}{payTo: 0.1}
	outJSON, _ := json.Marshal(outObj)
	rawHex, code, msg := execCreateRawTransaction("test", []json.RawMessage{inp, outJSON, json.RawMessage(`2`)})
	if code != 0 {
		t.Fatalf("create: %d %q", code, msg)
	}

	prevEntry := map[string]interface{}{
		"txid":         prevTxid,
		"vout":         0,
		"scriptPubKey": hex.EncodeToString(p2sh),
		"redeemScript": hex.EncodeToString(redeem),
	}
	prevArr, _ := json.Marshal([]map[string]interface{}{prevEntry})
	privArr, _ := json.Marshal([]string{wif})
	res, code2, msg2 := execSignRawTransaction("test", nil, []json.RawMessage{
		json.RawMessage(`"` + rawHex.(string) + `"`),
		prevArr,
		privArr,
		json.RawMessage(`"ALL"`),
	})
	if code2 != 0 || msg2 != "" {
		t.Fatalf("sign: %d %q", code2, msg2)
	}
	if !res["complete"].(bool) {
		t.Fatalf("complete=false errors=%v", res["errors"])
	}
}
