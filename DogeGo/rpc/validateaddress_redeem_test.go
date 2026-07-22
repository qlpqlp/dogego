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

func TestValidateAddressP2SHWithRedeemScript(t *testing.T) {
	k1, err := secp256k1.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pub := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	nReq, _ := json.Marshal(1)
	keys, _ := json.Marshal([]string{pub})
	msRes, code, msg := execCreateMultisig("test", []json.RawMessage{nReq, keys})
	if code != 0 {
		t.Fatalf("createmultisig: %d %q", code, msg)
	}
	addr := msRes["address"].(string)
	redeemHex := msRes["redeemScript"].(string)
	redeemJ, _ := json.Marshal(redeemHex)
	addrJ, _ := json.Marshal(addr)
	res, code2, msg2 := execValidateAddress("test", nil, []json.RawMessage{addrJ, redeemJ})
	if code2 != 0 || msg2 != "" {
		t.Fatalf("validate: %d %q", code2, msg2)
	}
	if !res["isvalid"].(bool) || !res["isscript"].(bool) {
		t.Fatalf("%v", res)
	}
	badRedeem, _ := hex.DecodeString(redeemHex)
	badRedeem[0] ^= 0xff
	badJ, _ := json.Marshal(hex.EncodeToString(badRedeem))
	res2, _, _ := execValidateAddress("test", nil, []json.RawMessage{addrJ, badJ})
	if res2["isvalid"].(bool) {
		t.Fatal("expected invalid for wrong redeem")
	}
}
