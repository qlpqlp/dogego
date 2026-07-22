// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"dogego/wire"
)

func testCoinbaseTx(t *testing.T) *wire.Tx {
	t.Helper()
	raw := minimalCoinbaseRaw(t)
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

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

func TestAcceptMempoolTxRejectsCoinbase(t *testing.T) {
	err := AcceptMempoolTx(testCoinbaseTx(t), nil)
	if !errors.Is(err, ErrMempoolCoinbase) {
		t.Fatalf("got %v", err)
	}
}

func TestCheckTransactionRejectsDuplicateInputs(t *testing.T) {
	tx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{
			{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff},
			{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff},
		},
		Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	if err := CheckTransaction(tx, true); err == nil {
		t.Fatal("expected duplicate input error")
	}
}
