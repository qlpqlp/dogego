// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"dogego/pow"
	"dogego/wire"
)

// TestBackfillAuxThroughHeightPrefixOnly fills height 0 while the header tip is far ahead (prefix rewrite).
func TestBackfillAuxThroughHeightPrefixOnly(t *testing.T) {
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
	h2 := append([]byte(nil), genesis...)
	binary.LittleEndian.PutUint32(h2[0:4], 2|(1<<8))
	genHash := pow.BlockHashLE(genesis)
	copy(h2[4:36], genHash[:])
	if err := j.AppendWireHeaderBatch(h2); err != nil {
		t.Fatal(err)
	}
	tip, _ := j.TipHeight()
	if tip != 1 {
		t.Fatalf("tip %d want 1", tip)
	}
	aux, err := OpenHeaderAuxJournal(filepath.Join(dir, "headers_aux.bin"), tip+1)
	if err != nil {
		t.Fatal(err)
	}
	rawStore, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	inner := encodeTestAuxPowBackfill(t)
	h80 := genesis
	var block bytes.Buffer
	_, _ = block.Write(h80)
	_, _ = block.Write(inner)
	hash := pow.BlockHashLE(h80)
	rbDir := filepath.Join(dir, "rawblocks")
	if err := os.MkdirAll(rbDir, 0o700); err != nil {
		t.Fatal(err)
	}
	rbPath := filepath.Join(rbDir, hex.EncodeToString(hash[:])+".bin")
	if err := os.WriteFile(rbPath, block.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := BackfillAuxThroughHeight(j, aux, rawStore, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("filled %d want 1", n)
	}
	got, err := aux.ReadAt(0)
	if err != nil || len(got) == 0 {
		t.Fatalf("aux at 0: len=%d err=%v", len(got), err)
	}
	empty, err := aux.ReadAt(1)
	if err != nil || len(empty) != 0 {
		t.Fatalf("aux at 1 should stay empty: len=%d err=%v", len(empty), err)
	}
	if aux.RecordCount() != tip+1 {
		t.Fatalf("record count %d want %d", aux.RecordCount(), tip+1)
	}
}

func encodeTestAuxPowBackfill(t *testing.T) []byte {
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
