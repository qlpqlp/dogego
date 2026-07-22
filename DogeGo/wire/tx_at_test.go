// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"encoding/binary"
	"testing"

	"dogego/primitives"
)

func minimalCoinbaseTxBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, int32(1))
	_ = WriteCompactSize(&buf, 1)
	var zeros [32]byte
	_, _ = buf.Write(zeros[:])
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = WriteCompactSize(&buf, 1)
	_, _ = buf.Write([]byte{0x00})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = WriteCompactSize(&buf, 1)
	_ = binary.Write(&buf, binary.LittleEndian, int64(8800000000))
	_ = WriteCompactSize(&buf, 2)
	_, _ = buf.Write([]byte{0x51, 0x51})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	return buf.Bytes()
}

func TestReadTxAtIndexCoinbase(t *testing.T) {
	cbRaw := minimalCoinbaseTxBytes(t)
	cb, err := DeserializeTx(cbRaw)
	if err != nil {
		t.Fatal(err)
	}
	mr := BlockMerkleRoot([]*Tx{cb})
	hdr := primitives.BlockHeader{Version: 1, MerkleRoot: mr, Timestamp: 1700000000, Bits: 0x1e0ffff0, Nonce: 1}
	h80 := hdr.EncodeWire80()
	var block bytes.Buffer
	_, _ = block.Write(h80[:])
	_ = WriteCompactSize(&block, 1)
	ser, _ := cb.Serialize()
	_, _ = block.Write(ser)
	raw := block.Bytes()
	tx, meta, err := ReadTxAtIndex(raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	if tx == nil || meta.Header.Timestamp != hdr.Timestamp {
		t.Fatalf("meta %#v", meta)
	}
}
