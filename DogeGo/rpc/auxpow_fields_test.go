// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/wire"
)

func TestAuxpowToJSONFromParsed(t *testing.T) {
	inner, _ := (&wire.Tx{Version: 1, Vout: []wire.TxOut{{Value: 0, PkScript: []byte{0x51}}}}).Serialize()
	ap := &wire.AuxPow{
		Coinbase:       &wire.Tx{Version: 1, Vout: []wire.TxOut{{Value: 0, PkScript: inner}}},
		MerkleIndex:    1,
		ChainIndex:     2,
		ParentHeader80: [80]byte{1},
	}
	obj, err := auxpowToJSON(ap)
	if err != nil || obj == nil {
		t.Fatalf("auxpow json: %v %v", obj, err)
	}
	if obj["index"] != int32(1) || obj["chainindex"] != int32(2) {
		t.Fatalf("indexes: %+v", obj)
	}
	txObj, ok := obj["tx"].(map[string]interface{})
	if !ok || txObj["txid"] == nil {
		t.Fatalf("missing tx object: %+v", obj)
	}
}

func TestAttachAuxPowFieldOnBlock(t *testing.T) {
	ap := &wire.AuxPow{
		Coinbase:       &wire.Tx{Version: 1, Vout: []wire.TxOut{{Value: 0, PkScript: []byte{0x51}}}},
		ParentHeader80: [80]byte{2},
	}
	pb := &wire.ParsedBlock{Aux: ap}
	m := map[string]interface{}{}
	attachAuxPowField(m, pb, nil, 0)
	if _, ok := m["auxpow"].(map[string]interface{}); !ok {
		t.Fatalf("expected auxpow object, got %T", m["auxpow"])
	}
}
