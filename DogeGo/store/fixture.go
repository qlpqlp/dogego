// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"bytes"
	"encoding/binary"
	"testing"

	"dogego/pow"
	"dogego/primitives"
	"dogego/wire"
)

// TestMinimalBlock returns a valid legacy block (single coinbase) and its block id (LE).
// Intended for unit tests in other packages.
func TestMinimalBlock() ([]byte, [32]byte) {
	var txb bytes.Buffer
	_ = binary.Write(&txb, binary.LittleEndian, int32(1))
	_ = wire.WriteCompactSize(&txb, 1)
	var zeros [32]byte
	_, _ = txb.Write(zeros[:])
	_ = binary.Write(&txb, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&txb, 2)
	_, _ = txb.Write([]byte{0x01, 0x00}) // coinbase script (non-empty; satisfies bad-cb-length)
	_ = binary.Write(&txb, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&txb, 1)
	_ = binary.Write(&txb, binary.LittleEndian, int64(8800000000))
	_ = wire.WriteCompactSize(&txb, 2)
	_, _ = txb.Write([]byte{0x51, 0x51})
	_ = binary.Write(&txb, binary.LittleEndian, uint32(0))
	tx, _ := wire.ReadTx(bytes.NewReader(txb.Bytes()))
	hdr := primitives.BlockHeader{
		Version:    1,
		PrevBlock:  [32]byte{},
		MerkleRoot: tx.TxHash(),
		Timestamp:  1747000000,
		Bits:       0x1e0ffff0,
		Nonce:      2139303,
	}
	var block bytes.Buffer
	h80 := hdr.EncodeWire80()
	_, _ = block.Write(h80[:])
	_ = wire.WriteCompactSize(&block, 1)
	_, _ = block.Write(txb.Bytes())
	return block.Bytes(), pow.BlockHashLE(h80[:])
}

// MakeTestBlockRaw builds a minimal single-tx block with merkle root set in h80.
func MakeTestBlockRaw(t *testing.T, h80 []byte) []byte {
	t.Helper()
	return makeTestBlockRaw(t, h80)
}

func makeTestBlockRaw(t *testing.T, h80 []byte) []byte {
	t.Helper()
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Script: []byte{2, 0}}},
		Vout:    []wire.TxOut{{Value: 50e8, PkScript: []byte{0x51}}},
	}
	txRaw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	hdr := append([]byte(nil), h80...)
	root := wire.BlockMerkleRoot([]*wire.Tx{tx})
	copy(hdr[36:68], root[:])
	var buf []byte
	buf = append(buf, hdr...)
	buf = append(buf, 1)
	buf = append(buf, txRaw...)
	return buf
}
