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
)

func TestDecodeScriptMultisigRedeem(t *testing.T) {
	k, err := secp256k1.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pub := hex.EncodeToString(k.PubKey().SerializeCompressed())
	nReq, _ := json.Marshal(1)
	keys, _ := json.Marshal([]string{pub})
	res, code, msg := execCreateMultisig("test", []json.RawMessage{nReq, keys})
	if code != 0 {
		t.Fatalf("createmultisig: %d %q", code, msg)
	}
	redeemHex := res["redeemScript"].(string)
	raw, _ := json.Marshal(redeemHex)
	out, code2, msg2 := execDecodeScript("test", []json.RawMessage{raw})
	if code2 != 0 || msg2 != "" {
		t.Fatalf("decodescript: %d %q", code2, msg2)
	}
	if out["type"] != "multisig" {
		t.Fatalf("type %v", out["type"])
	}
	if out["reqSigs"].(int) != 1 {
		t.Fatalf("reqSigs %v", out["reqSigs"])
	}
	addrs, _ := out["addresses"].([]interface{})
	if len(addrs) != 1 {
		t.Fatalf("addresses %v", addrs)
	}
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	h := pubkeyHash160(k.PubKey().SerializeCompressed())
	want := chain.Base58CheckEncode(p.PubkeyHashAddrID, h[:])
	if addrs[0] != want {
		t.Fatalf("addr %v want %s", addrs[0], want)
	}
}
