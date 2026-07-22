// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"dogego/wire"
)

func TestPool_GetRawByTxID(t *testing.T) {
	p := New(10)
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
	raw := buf.Bytes()
	if err := p.Add(raw); err != nil {
		t.Fatal(err)
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := txidDisplayHex(tx.TxHash())
	got, err := p.GetRawByTxID(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatal("raw mismatch")
	}
	if _, err := p.GetRawByTxID(strings.Repeat("g", 64)); err == nil {
		t.Fatal("bad hex want error")
	}
}
