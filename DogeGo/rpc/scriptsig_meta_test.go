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
	"dogego/wire"
)

func TestScriptSigRPCNestedRedeemPushes(t *testing.T) {
	inner := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	inner = append(inner, 0x88, 0xac)
	h := chain.Hash160(inner)
	forward := append([]byte{0xa9, 0x14}, h...)
	forward = append(forward, 0x87)
	script, err := concatPushesForTest([]byte{0xab}, inner, forward)
	if err != nil {
		t.Fatal(err)
	}
	out := scriptSigRPC(script)
	pushes, ok := out["dogego_redeem_pushes"].([]interface{})
	if !ok || len(pushes) != 2 {
		t.Fatalf("pushes %#v", out["dogego_redeem_pushes"])
	}
	last, ok := out["dogego_redeem"].(map[string]interface{})
	if !ok || last["dogego_script_template"] != "p2sh_forward" {
		t.Fatalf("last %#v", out["dogego_redeem"])
	}
}

func concatPushesForTest(parts ...[]byte) ([]byte, error) {
	var out []byte
	for _, p := range parts {
		chunk, err := pushScriptData(p)
		if err != nil {
			return nil, err
		}
		out = append(out, chunk...)
	}
	return out, nil
}

func TestDecoderawtransactionScriptSigRedeemPushes(t *testing.T) {
	inner := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	inner[3] = 0x55
	inner = append(inner, 0x88, 0xac)
	h := chain.Hash160(inner)
	forward := append([]byte{0xa9, 0x14}, h...)
	forward = append(forward, 0x87)
	script, _ := concatPushesForTest(inner, forward)
	tx := minimalTxWithScriptSig(script)
	ser, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(hex.EncodeToString(ser))
	res, code, msg := execDecodeRawTransaction("test", []json.RawMessage{raw})
	if code != 0 || msg != "" {
		t.Fatalf("decode: %d %q", code, msg)
	}
	m := res.(map[string]interface{})
	vin := m["vin"].([]interface{})
	in0 := vin[0].(map[string]interface{})
	sig, _ := in0["scriptSig"].(map[string]interface{})
	if sig["dogego_redeem_pushes"] == nil {
		t.Fatalf("scriptSig %#v", sig)
	}
}

func minimalTxWithScriptSig(script []byte) *wire.Tx {
	return &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{2},
			PrevIdx:  0,
			Sequence: 0xffffffff,
			Script:   script,
		}},
		Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
}
