// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func TestExecSubmitBlockStoresKnownHeader(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	blockRaw := makeSubmitTestBlock(t, g80[:])
	hdr := blockRaw[:80]
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), hdr...)}}
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	hexJ, _ := json.Marshal(hex.EncodeToString(blockRaw))
	res, code, msg := execSubmitBlock(j, nil, raw, nil, "test", []json.RawMessage{hexJ})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q res=%v", code, msg, res)
	}
	if !raw.Has(pow.BlockHashLE(hdr)) {
		t.Fatal("block not stored")
	}
}

func TestExecSubmitBlockRelays(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	blockRaw := makeSubmitTestBlock(t, g80[:])
	hdr := blockRaw[:80]
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), hdr...)}}
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	var relayed []byte
	hexJ, _ := json.Marshal(hex.EncodeToString(blockRaw))
	paths := &DataPaths{RelayBlock: func(b []byte) error {
		relayed = append([]byte(nil), b...)
		return nil
	}}
	res, code, msg := execSubmitBlock(j, nil, raw, paths, "test", []json.RawMessage{hexJ})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q res=%v", code, msg, res)
	}
	if len(relayed) != len(blockRaw) {
		t.Fatalf("relayed len %d", len(relayed))
	}
}

func TestExecSubmitBlockExtendsTipRejectsInvalid(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(dir+"/headers.bin", g80[:])
	if err != nil {
		t.Fatal(err)
	}
	genHash := pow.BlockHashLE(g80[:])
	h1 := append([]byte(nil), g80[:]...)
	copy(h1[4:36], genHash[:])
	h1[76] ^= 0x33
	blockRaw := store.MakeTestBlockRaw(t, h1)
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	hexJ, _ := json.Marshal(hex.EncodeToString(blockRaw))
	res, code, msg := execSubmitBlock(j, nil, raw, nil, "test", []json.RawMessage{hexJ})
	if code != 0 || res == nil {
		t.Fatalf("code=%d msg=%q res=%v", code, msg, res)
	}
	if !strings.Contains(res.(string), "rejected") {
		t.Fatalf("want rejection, got %v", res)
	}
	if tip, _ := j.TipHeight(); tip != 0 {
		t.Fatalf("tip=%d", tip)
	}
}

func makeSubmitTestBlock(t *testing.T, h80 []byte) []byte {
	t.Helper()
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Script: []byte{2, 0}}},
		Vout:    []wire.TxOut{{Value: 50e8, PkScript: []byte{0x51}}},
	}
	txRaw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	hdr := append([]byte(nil), h80...)
	root := wire.BlockMerkleRoot([]*wire.Tx{tx})
	copy(hdr[36:68], root[:])
	var buf []byte
	buf = append(buf, hdr...)
	buf = append(buf, 1)
	buf = append(buf, txRaw...)
	return buf
}
