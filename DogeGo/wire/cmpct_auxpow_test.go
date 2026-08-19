// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestBuildHeaderAndShortIDsFromBlock_rejectsAuxpow(t *testing.T) {
	raw := minimalAuxpowBlockRaw(t)
	_, err := BuildHeaderAndShortIDsFromBlock(raw, 1)
	if err == nil {
		t.Fatal("expected auxpow cmpct build error")
	}
}

func TestReconstructBlockFromCmpct_rejectsAuxpow(t *testing.T) {
	// Synthetic HeaderAndShortIDs with AuxPoW version bit â€” reconstruct must refuse
	// (cmpct cannot carry the aux blob). Use a minimal valid coinbase as prefilled tx 0.
	var coin bytes.Buffer
	_ = binary.Write(&coin, binary.LittleEndian, int32(1))
	_ = WriteCompactSize(&coin, 1)
	var zeros [32]byte
	_, _ = coin.Write(zeros[:])
	_ = binary.Write(&coin, binary.LittleEndian, uint32(0xffffffff))
	_ = WriteCompactSize(&coin, 1)
	_, _ = coin.Write([]byte{0x00})
	_ = binary.Write(&coin, binary.LittleEndian, uint32(0xffffffff))
	_ = WriteCompactSize(&coin, 1)
	_ = binary.Write(&coin, binary.LittleEndian, int64(1))
	_ = WriteCompactSize(&coin, 1)
	_, _ = coin.Write([]byte{0x51})
	_ = binary.Write(&coin, binary.LittleEndian, uint32(0))

	var hdr [80]byte
	binary.LittleEndian.PutUint32(hdr[0:4], 1|(1<<8))
	hs := &HeaderAndShortIDs{
		Header80: hdr,
		Nonce:    1,
		Prefilled: []PrefilledTransaction{
			{Index: 0, Tx: coin.Bytes()},
		},
	}
	_, err := ReconstructBlockFromCmpct(hs, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "auxpow") {
		t.Fatalf("want auxpow reconstruct error, got %v", err)
	}
}

func minimalAuxpowBlockRaw(t *testing.T) []byte {
	t.Helper()
	inner := minimalAuxPowBytes(t)
	hdr := make([]byte, 80)
	binary.LittleEndian.PutUint32(hdr[0:4], 1|(1<<8))
	var coinbase bytes.Buffer
	_ = WriteCompactSize(&coinbase, 1)
	var z [32]byte
	_, _ = coinbase.Write(z[:])
	_ = binary.Write(&coinbase, binary.LittleEndian, uint32(0xffffffff))
	_ = WriteCompactSize(&coinbase, 1)
	_, _ = coinbase.Write([]byte{0x00})
	_ = WriteCompactSize(&coinbase, 1)
	_ = binary.Write(&coinbase, binary.LittleEndian, int64(1))
	_ = WriteCompactSize(&coinbase, 1)
	_, _ = coinbase.Write([]byte{0x51})
	_ = binary.Write(&coinbase, binary.LittleEndian, uint32(0))
	var block bytes.Buffer
	_, _ = block.Write(hdr)
	_, _ = block.Write(inner)
	_ = WriteCompactSize(&block, 1)
	_, _ = block.Write(coinbase.Bytes())
	return block.Bytes()
}
