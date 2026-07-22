// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestScriptSigRedeemMetasNestedP2SH(t *testing.T) {
	inner := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	inner = append(inner, 0x88, 0xac)
	forward := p2shScriptPubKey(inner)
	script, err := concatScriptPushes([]byte{0x01}, []byte{0x02}, inner, forward)
	if err != nil {
		t.Fatal(err)
	}
	metas := ScriptSigRedeemMetas(script)
	if len(metas) != 2 {
		t.Fatalf("metas %d", len(metas))
	}
	if metas[0]["dogego_script_template"] != "pubkeyhash" {
		t.Fatalf("inner %v", metas[0])
	}
	if metas[1]["dogego_script_template"] != "p2sh_forward" {
		t.Fatalf("forward %v", metas[1])
	}
}
