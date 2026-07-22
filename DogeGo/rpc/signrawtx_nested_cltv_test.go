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

func TestExecSignRawTransactionNestedP2SHCLTVP2PKH(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	sec := make([]byte, 32)
	sec[0] = 0x19
	wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	pubC := mustPubCompressed(t, sec)
	h160 := pubkeyHash160(pubC)
	lockHeight := int64(700)
	cltvRedeem := consensus.BuildCLTVP2PKHRedeemScript(lockHeight, h160)
	innerH := hash160P2SH(cltvRedeem)
	forward := append([]byte{0xa9, 0x14}, innerH[:]...)
	forward = append(forward, 0x87)
	fwdH := hash160P2SH(forward)
	outerP2SH := append([]byte{0xa9, 0x14}, fwdH[:]...)
	outerP2SH = append(outerP2SH, 0x87)
	payTo, _ := chain.RandomP2PKHAddress(p)

	prevTxid := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	inp, _ := json.Marshal([]map[string]interface{}{{"txid": prevTxid, "vout": 0, "sequence": 0xfffffffe}})
	outJSON, _ := json.Marshal(map[string]interface{}{payTo: 0.1})
	lockJSON, _ := json.Marshal(float64(lockHeight))
	rawHex, code, msg := execCreateRawTransaction("test", []json.RawMessage{inp, outJSON, lockJSON})
	if code != 0 {
		t.Fatalf("create: %d %q", code, msg)
	}

	prevEntry := map[string]interface{}{
		"txid":              prevTxid,
		"vout":              0,
		"scriptPubKey":      hex.EncodeToString(outerP2SH),
		"redeemScript":      hex.EncodeToString(forward),
		"innerRedeemScript": hex.EncodeToString(cltvRedeem),
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
