// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"dogego/pow"
	"dogego/primitives"
	"dogego/wire"
)

func buildMinimalCoinbase(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, int32(1))
	_ = wire.WriteCompactSize(&buf, 1)
	var zeros [32]byte
	_, _ = buf.Write(zeros[:])
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&buf, 1)
	_, _ = buf.Write([]byte{0x00})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&buf, 1)
	_ = binary.Write(&buf, binary.LittleEndian, int64(8800000000))
	_ = wire.WriteCompactSize(&buf, 2)
	_, _ = buf.Write([]byte{0x51, 0x51})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	return buf.Bytes()
}

func TestParseBlockValidateParsedBlock(t *testing.T) {
	txb := buildMinimalCoinbase(t)
	rt := bytes.NewReader(txb)
	tx, err := wire.ReadTx(rt)
	if err != nil {
		t.Fatal(err)
	}
	th := tx.TxHash()
	hdr := primitives.BlockHeader{
		Version:    1,
		PrevBlock:  [32]byte{},
		MerkleRoot: th,
		Timestamp:  1747000000,
		Bits:       0x1e0ffff0,
		Nonce:      2139303,
	}
	var block bytes.Buffer
	h80 := hdr.EncodeWire80()
	_, _ = block.Write(h80[:])
	_ = wire.WriteCompactSize(&block, 1)
	_, _ = block.Write(txb)

	pb, err := wire.ParseBlock(block.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(pb.Txs) != 1 {
		t.Fatalf("txs %d", len(pb.Txs))
	}
	var foreachCount int
	if err := wire.ForEachBlockTx(block.Bytes(), func(i uint32, tx *wire.Tx) error {
		foreachCount++
		if tx.Version != 1 {
			t.Fatalf("version %d", tx.Version)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if foreachCount != 1 {
		t.Fatalf("foreach count %d", foreachCount)
	}
	nTx, err := wire.BlockTxCount(block.Bytes())
	if err != nil || nTx != 1 {
		t.Fatalf("BlockTxCount err=%v n=%d", err, nTx)
	}
	wantID := pow.BlockHashLE(h80[:])
	if err := wire.ValidateParsedBlock(pb, wantID); err != nil {
		t.Fatal(err)
	}
}

func TestParseBlockBadMerkle(t *testing.T) {
	txb := buildMinimalCoinbase(t)
	hdr := primitives.BlockHeader{
		Version:    1,
		PrevBlock:  [32]byte{},
		MerkleRoot: [32]byte{1, 2, 3},
		Timestamp:  1,
		Bits:       0x1e0ffff0,
		Nonce:      2,
	}
	var block bytes.Buffer
	h80 := hdr.EncodeWire80()
	_, _ = block.Write(h80[:])
	_ = wire.WriteCompactSize(&block, 1)
	_, _ = block.Write(txb)
	pb, err := wire.ParseBlock(block.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if wire.VerifyBlockMerkle(pb) == nil {
		t.Fatal("expected merkle error")
	}
}

func buildMinimalCoinbaseVariant(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, int32(1))
	_ = wire.WriteCompactSize(&buf, 1)
	var zeros [32]byte
	_, _ = buf.Write(zeros[:])
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&buf, 1)
	_, _ = buf.Write([]byte{0x01}) // different script from buildMinimalCoinbase
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&buf, 1)
	_ = binary.Write(&buf, binary.LittleEndian, int64(8800000000))
	_ = wire.WriteCompactSize(&buf, 2)
	_, _ = buf.Write([]byte{0x51, 0x51})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	return buf.Bytes()
}

func encodeAuxPowPayload(innerTx []byte) []byte {
	var b bytes.Buffer
	_, _ = b.Write(innerTx)
	var z [32]byte
	_, _ = b.Write(z[:])
	_ = wire.WriteCompactSize(&b, 0)
	_ = binary.Write(&b, binary.LittleEndian, int32(-1))
	_ = wire.WriteCompactSize(&b, 0)
	_ = binary.Write(&b, binary.LittleEndian, int32(0))
	var parent [80]byte
	_, _ = b.Write(parent[:])
	return b.Bytes()
}

func TestParseBlockWithAuxPow(t *testing.T) {
	mainTx := buildMinimalCoinbase(t)
	rt := bytes.NewReader(mainTx)
	tx, err := wire.ReadTx(rt)
	if err != nil {
		t.Fatal(err)
	}
	th := tx.TxHash()
	hdr := primitives.BlockHeader{
		Version:    1 | (1 << 8),
		PrevBlock:  [32]byte{},
		MerkleRoot: th,
		Timestamp:  1747000000,
		Bits:       0x1e0ffff0,
		Nonce:      2139303,
	}
	h80 := hdr.EncodeWire80()
	inner := buildMinimalCoinbaseVariant(t)
	var block bytes.Buffer
	_, _ = block.Write(h80[:])
	_, _ = block.Write(encodeAuxPowPayload(inner))
	_ = wire.WriteCompactSize(&block, 1)
	_, _ = block.Write(mainTx)

	pb, err := wire.ParseBlock(block.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(pb.Txs) != 1 {
		t.Fatalf("txs %d", len(pb.Txs))
	}
	wantID := pow.BlockHashLE(h80[:])
	if err := wire.ValidateParsedBlock(pb, wantID); err != nil {
		t.Fatal(err)
	}
}
