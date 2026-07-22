// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/wire"
)

func TestShouldInlinePatchAux(t *testing.T) {
	if !shouldInlinePatchAux(chain.MainnetDogecoin, 371337, 5_000_000, 371300) {
		t.Fatal("activation window with bodies at fork should allow inline patch")
	}
	if shouldInlinePatchAux(chain.MainnetDogecoin, 371337, 5_000_000, 1000) {
		t.Fatal("activation height with bodies far below should defer to batch backfill")
	}
	if shouldInlinePatchAux(chain.MainnetDogecoin, 1000, 5_000_000, 3_000_100) {
		t.Fatal("ancient height far from body frontier should defer to batch backfill")
	}
	if !shouldInlinePatchAux(chain.MainnetDogecoin, 3_000_050, 5_000_000, 3_000_100) {
		t.Fatal("height near contiguous frontier should allow inline patch")
	}
	if !shouldInlinePatchAux(chain.MainnetDogecoin, 4_999_900, 5_000_000, 4_999_800) {
		t.Fatal("near header tip should allow inline patch")
	}
}

func TestPatchRecordAt(t *testing.T) {
	dir := t.TempDir()
	p, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesis := append([]byte(nil), p[:]...)
	binary.LittleEndian.PutUint32(genesis[0:4], 1|(1<<8))
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesis)
	if err != nil {
		t.Fatal(err)
	}
	aux, err := OpenHeaderAuxJournal(filepath.Join(dir, "headers_aux.bin"), 1)
	if err != nil {
		t.Fatal(err)
	}
	inner := encodeTestAuxPowPatch(t)
	var block bytes.Buffer
	_, _ = block.Write(genesis)
	_, _ = block.Write(inner)
	ok, err := PatchAuxFromBlockAtHeight(j, aux, chain.MainnetDogecoin, 0, 0, block.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected patch")
	}
	got, err := aux.ReadAt(0)
	if err != nil || len(got) == 0 {
		t.Fatalf("read aux: len=%d err=%v", len(got), err)
	}
	ok2, err := PatchAuxFromBlockAtHeight(j, aux, chain.MainnetDogecoin, 0, 0, block.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if ok2 {
		t.Fatal("second patch should be no-op")
	}
}

func encodeTestAuxPowPatch(t *testing.T) []byte {
	t.Helper()
	var coinbase []byte
	{
		var cb bytes.Buffer
		_ = wire.WriteCompactSize(&cb, 1)
		var z [32]byte
		_, _ = cb.Write(z[:])
		_ = binary.Write(&cb, binary.LittleEndian, uint32(0xffffffff))
		_ = wire.WriteCompactSize(&cb, 1)
		_, _ = cb.Write([]byte{0x00})
		_ = wire.WriteCompactSize(&cb, 1)
		_ = binary.Write(&cb, binary.LittleEndian, int64(1))
		_ = wire.WriteCompactSize(&cb, 1)
		_, _ = cb.Write([]byte{0x51})
		_ = binary.Write(&cb, binary.LittleEndian, uint32(0))
		coinbase = cb.Bytes()
	}
	var b bytes.Buffer
	_, _ = b.Write(coinbase)
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
