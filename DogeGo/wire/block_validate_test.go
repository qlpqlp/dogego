// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire_test

import (
	"bytes"
	"testing"

	"dogego/pow"
	"dogego/primitives"
	"dogego/wire"
)

func TestValidateBlockPayloadMatchesParseBlock(t *testing.T) {
	txb := buildMinimalCoinbase(t)
	rt := bytes.NewReader(txb)
	tx, err := wire.ReadTx(rt)
	if err != nil {
		t.Fatal(err)
	}
	th := tx.TxHash()
	hdr := primitives.BlockHeader{
		Version:    1,
		MerkleRoot: th,
		Timestamp:  1747000000,
		Bits:       0x1e0ffff0,
		Nonce:      2139303,
	}
	var block bytes.Buffer
	h80 := hdr.EncodeWire80()
	block.Write(h80[:])
	_ = wire.WriteCompactSize(&block, 1)
	block.Write(txb)
	raw := block.Bytes()
	want := pow.BlockHashLE(h80[:])
	if err := wire.ValidateBlockPayload(raw, want); err != nil {
		t.Fatal(err)
	}
	ids, err := wire.RPCTxidsFromPayload(raw)
	if err != nil || len(ids) != 1 {
		t.Fatalf("txids %v err %v", ids, err)
	}
	tx, idx, err := wire.FindTxByRPCID(raw, ids[0])
	if err != nil || idx != 0 || tx == nil {
		t.Fatalf("find tx idx=%d err=%v", idx, err)
	}
}
