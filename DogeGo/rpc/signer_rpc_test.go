// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"
)

func TestExecEnumerateSignersUnset(t *testing.T) {
	res, code, msg := execEnumerateSigners(nil, nil)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	list, ok := res.([]interface{})
	if !ok || len(list) != 0 {
		t.Fatalf("expected empty list, got %#v", res)
	}
}

func TestExecEnumerateSignersWrongArgs(t *testing.T) {
	_, code, _ := execEnumerateSigners(nil, []json.RawMessage{json.RawMessage(`1`)})
	if code != -32602 {
		t.Fatalf("code=%d", code)
	}
}

func TestExecSignerDisplayAddressNoSigner(t *testing.T) {
	_, code, msg := execSignerDisplayAddress(nil, mustWalletJSONParam(t, "pkh(02abc)"))
	if code != -1 {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExternalSignerClientNil(t *testing.T) {
	if externalSignerClient(nil) != nil {
		t.Fatal("nil paths")
	}
	if externalSignerClient(&DataPaths{}) != nil {
		t.Fatal("empty signer command")
	}
}
