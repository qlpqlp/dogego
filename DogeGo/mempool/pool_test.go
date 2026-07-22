// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"bytes"
	"encoding/binary"
	"testing"

	"dogego/wire"
)

func minimalCoinbaseRaw(t *testing.T) []byte {
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

func secondDistinctRaw(t *testing.T, first []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, int32(2))
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
	out := buf.Bytes()
	if bytes.Equal(out, first) {
		t.Fatal("expected distinct raw tx")
	}
	return out
}

func TestPoolRawMemPoolTxIDsEmpty(t *testing.T) {
	p := New(10)
	ids, err := p.RawMemPoolTxIDs()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("got %v", ids)
	}
	if p.TotalBytes() != 0 {
		t.Fatalf("bytes %d", p.TotalBytes())
	}
}

func TestPoolRawMemPoolTxIDsSorted(t *testing.T) {
	p := New(10)
	a := minimalCoinbaseRaw(t)
	b := secondDistinctRaw(t, a)
	if err := p.Add(b); err != nil {
		t.Fatal(err)
	}
	if err := p.Add(a); err != nil {
		t.Fatal(err)
	}
	ids, err := p.RawMemPoolTxIDs()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("len %d", len(ids))
	}
	if ids[0] >= ids[1] {
		t.Fatalf("not sorted: %q %q", ids[0], ids[1])
	}
	wantBytes := len(a) + len(b)
	if p.TotalBytes() != wantBytes {
		t.Fatalf("TotalBytes got %d want %d", p.TotalBytes(), wantBytes)
	}
}

func TestPoolContainsTxIDAndIsFull(t *testing.T) {
	p := New(1)
	if p.IsFull() {
		t.Fatal("empty should not be full")
	}
	a := minimalCoinbaseRaw(t)
	tx, err := wire.DeserializeTx(a)
	if err != nil {
		t.Fatal(err)
	}
	id := txidDisplayHex(tx.TxHash())
	if p.ContainsTxID(id) {
		t.Fatal("missing tx")
	}
	if err := p.Add(a); err != nil {
		t.Fatal(err)
	}
	if !p.ContainsTxID(id) {
		t.Fatal("expected contains")
	}
	if !p.IsFull() {
		t.Fatal("expected full")
	}
}
