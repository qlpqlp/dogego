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
)

func TestExtractAuxPowBytesFromBlock(t *testing.T) {
	inner := minimalAuxPowBytes(t)
	hdr := make([]byte, 80)
	binary.LittleEndian.PutUint32(hdr[0:4], 1|(1<<8))
	var block bytes.Buffer
	_, _ = block.Write(hdr)
	_, _ = block.Write(inner)

	got, ok, err := ExtractAuxPowBytesFromBlock(block.Bytes())
	if err != nil || !ok || len(got) == 0 {
		t.Fatalf("extract: ok=%v err=%v len=%d", ok, err, len(got))
	}
	_, err = ReadAuxPow(bytes.NewReader(got))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := ExtractAuxPowBytesFromBlock(hdr); ok {
		t.Fatal("expected no aux on legacy-only header bytes")
	}
}

func TestHeaderHasAuxPowVersion(t *testing.T) {
	h := make([]byte, 80)
	if HeaderHasAuxPowVersion(h) {
		t.Fatal("legacy")
	}
	binary.LittleEndian.PutUint32(h[0:4], 1|(1<<8))
	if !HeaderHasAuxPowVersion(h) {
		t.Fatal("aux")
	}
}

// minimalAuxPowBytes matches store/header_aux_backfill_test.go encodeTestAuxPow (valid ReadAuxPow).
func minimalAuxPowBytes(t *testing.T) []byte {
	t.Helper()
	var coinbase []byte
	{
		var cb bytes.Buffer
		_ = WriteCompactSize(&cb, 1)
		var z [32]byte
		_, _ = cb.Write(z[:])
		_ = binary.Write(&cb, binary.LittleEndian, uint32(0xffffffff))
		_ = WriteCompactSize(&cb, 1)
		_, _ = cb.Write([]byte{0x00})
		_ = WriteCompactSize(&cb, 1)
		_ = binary.Write(&cb, binary.LittleEndian, int64(1))
		_ = WriteCompactSize(&cb, 1)
		_, _ = cb.Write([]byte{0x51})
		_ = binary.Write(&cb, binary.LittleEndian, uint32(0))
		coinbase = cb.Bytes()
	}
	var b bytes.Buffer
	_, _ = b.Write(coinbase)
	var z [32]byte
	_, _ = b.Write(z[:])
	_ = WriteCompactSize(&b, 0)
	_ = binary.Write(&b, binary.LittleEndian, int32(-1))
	_ = WriteCompactSize(&b, 0)
	_ = binary.Write(&b, binary.LittleEndian, int32(0))
	var parent [80]byte
	_, _ = b.Write(parent[:])
	return b.Bytes()
}
