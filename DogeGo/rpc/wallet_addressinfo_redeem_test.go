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
)

func TestGetAddressInfoP2SHWithRedeemScript(t *testing.T) {
	k, err := secp256k1.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pub := hex.EncodeToString(k.PubKey().SerializeCompressed())
	nReq, _ := json.Marshal(1)
	keys, _ := json.Marshal([]string{pub})
	msRes, code, msg := execCreateMultisig("test", []json.RawMessage{nReq, keys})
	if code != 0 {
		t.Fatalf("createmultisig: %d %q", code, msg)
	}
	addrJ, _ := json.Marshal(msRes["address"])
	redeemJ, _ := json.Marshal(msRes["redeemScript"])
	res, code2, msg2 := execGetAddressInfo("test", nil, []json.RawMessage{addrJ, redeemJ})
	if code2 != 0 || msg2 != "" {
		t.Fatalf("getaddressinfo: %d %q", code2, msg2)
	}
	m := res.(map[string]interface{})
	if !m["isscript"].(bool) {
		t.Fatalf("%#v", m)
	}
}
