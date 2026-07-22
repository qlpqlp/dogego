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

	"dogego/wire"
)

func buildMinimalCoinbaseTx(t *testing.T) *wire.Tx {
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
	tx, err := wire.DeserializeTx(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

func TestTxSerializeLegacyRoundTrip(t *testing.T) {
	tx := buildMinimalCoinbaseTx(t)
	ser, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	tx2, err := wire.DeserializeTx(ser)
	if err != nil {
		t.Fatal(err)
	}
	if tx2.Version != tx.Version || tx2.LockTime != tx.LockTime {
		t.Fatalf("meta mismatch")
	}
	if len(tx2.Vin) != 1 || len(tx2.Vout) != 1 {
		t.Fatalf("counts %d %d", len(tx2.Vin), len(tx2.Vout))
	}
}

func TestWTxHashEqualsTxHashLegacy(t *testing.T) {
	tx := buildMinimalCoinbaseTx(t)
	if tx.TxHash() != tx.WTxHash() {
		t.Fatalf("legacy txid != wtxid")
	}
}

func TestTxWitnessSerializeRoundTrip(t *testing.T) {
	tx := buildMinimalCoinbaseTx(t)
	tx.Vin[0].Witness = [][]byte{{0x01, 0x02}, {0x03}}
	ser, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	tx2, err := wire.DeserializeTx(ser)
	if err != nil {
		t.Fatal(err)
	}
	if len(tx2.Vin) != 1 || len(tx2.Vin[0].Witness) != 2 {
		t.Fatalf("witness len %d", len(tx2.Vin[0].Witness))
	}
	if string(tx2.Vin[0].Witness[0]) != "\x01\x02" || string(tx2.Vin[0].Witness[1]) != "\x03" {
		t.Fatalf("witness bytes")
	}
	if tx.TxHash() == tx.WTxHash() {
		t.Fatal("expected txid != wtxid for witness tx")
	}
}
