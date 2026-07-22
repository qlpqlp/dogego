// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"testing"

	"dogego/pow"
	"dogego/primitives"
	"dogego/wire"
)

func TestValidateGBTProposalInconclusivePrev(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	tipPrev := pow.BlockHashLE(g80[:])
	coinbase := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 50, PkScript: []byte{0x51}}},
	}
	merkle := wire.BlockMerkleRoot([]*wire.Tx{coinbase})
	hdr := primitives.BlockHeader{
		Version: 1, PrevBlock: [32]byte{1, 2, 3}, MerkleRoot: merkle,
		Timestamp: 1_500_000_000, Bits: 0x1e0ffff0, Nonce: 1,
	}
	_ = tipPrev
	var buf bytes.Buffer
	h80b := hdr.EncodeWire80()
	_, _ = buf.Write(h80b[:])
	_ = wire.WriteCompactSize(&buf, 1)
	raw, _ := coinbase.Serialize()
	_, _ = buf.Write(raw)
	hexData := bytesToHex(buf.Bytes())
	res := validateGBTProposal(j, nil, nil, nil, nil, "main", 0, nil, hexData)
	if res != "inconclusive-not-best-prevblk" {
		t.Fatalf("got %#v", res)
	}
}

func bytesToHex(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexdigits[v>>4]
		out[i*2+1] = hexdigits[v&0x0f]
	}
	return string(out)
}
