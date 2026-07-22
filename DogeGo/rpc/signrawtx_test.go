// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"dogego/secp256k1"
	"golang.org/x/crypto/ripemd160"

	"dogego/chain"
)

func TestExecSignRawTransactionP2PKH(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	sec := make([]byte, 32)
	sec[0] = 0x11
	wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	payTo, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}

	prevTxid := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	inp, err := json.Marshal([]map[string]interface{}{{"txid": prevTxid, "vout": 0}})
	if err != nil {
		t.Fatal(err)
	}
	outObj := map[string]interface{}{payTo: 0.25}
	outJSON, err := json.Marshal(outObj)
	if err != nil {
		t.Fatal(err)
	}
	rawHex, code, msg := execCreateRawTransaction("test", []json.RawMessage{inp, outJSON})
	if code != 0 || msg != "" {
		t.Fatalf("createrawtransaction code=%d msg=%q", code, msg)
	}

	pubC := mustPubCompressed(t, sec)
	h160 := pubkeyHash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)
	prevEntry := map[string]interface{}{
		"txid":           prevTxid,
		"vout":           0,
		"scriptPubKey": hex.EncodeToString(pkScript),
	}
	prevArr, err := json.Marshal([]map[string]interface{}{prevEntry})
	if err != nil {
		t.Fatal(err)
	}
	privArr, err := json.Marshal([]string{wif})
	if err != nil {
		t.Fatal(err)
	}

	res, code2, msg2 := execSignRawTransaction("test", nil, []json.RawMessage{
		json.RawMessage(`"` + rawHex.(string) + `"`),
		prevArr,
		privArr,
		json.RawMessage(`"ALL"`),
	})
	if code2 != 0 || msg2 != "" {
		t.Fatalf("signrawtransaction code=%d msg=%q", code2, msg2)
	}
	if !res["complete"].(bool) {
		t.Fatalf("complete=false errors=%v", res["errors"])
	}
}

func TestExecSignRawTransactionP2SHP2PKH(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	sec := make([]byte, 32)
	sec[0] = 0x12
	wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	pubC := mustPubCompressed(t, sec)
	h160 := pubkeyHash160(pubC)
	redeem := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	redeem = append(redeem, 0x88, 0xac)
	rh := hash160P2SH(redeem)
	p2sh := append([]byte{0xa9, 0x14}, rh[:]...)
	p2sh = append(p2sh, 0x87)
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
		t.Fatalf("create: %d %q", code, msg)
	}

	prevEntry := map[string]interface{}{
		"txid":           prevTxid,
		"vout":           0,
		"scriptPubKey":   hex.EncodeToString(p2sh),
		"redeemScript":   hex.EncodeToString(redeem),
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

func hash160P2SH(script []byte) [20]byte {
	s := sha256.Sum256(script)
	r := ripemd160.New()
	_, _ = r.Write(s[:])
	var out [20]byte
	copy(out[:], r.Sum(nil))
	return out
}

func mustPubCompressed(t *testing.T, sec []byte) []byte {
	t.Helper()
	_, pub := secp256k1.PrivKeyFromBytes(sec)
	return pub.SerializeCompressed()
}
