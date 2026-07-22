// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dogego/pow"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

// TestExecReindexBlockFiltersRebuildsPersistedFilters verifies operator reindexblockfilters.
func TestExecReindexBlockFiltersRebuildsPersistedFilters(t *testing.T) {
	txb := minimalCoinbaseTxBytes(t)
	cbTx, err := wire.DeserializeTx(txb)
	if err != nil {
		t.Fatal(err)
	}
	mr0 := wire.BlockMerkleRoot([]*wire.Tx{cbTx})
	hdr0 := primitives.BlockHeader{Version: 1, MerkleRoot: mr0, Timestamp: 1700000000, Bits: 0x1e0ffff0, Nonce: 42}
	h0 := hdr0.EncodeWire80()
	id0 := pow.BlockHashLE(h0[:])
	var block0 bytes.Buffer
	_, _ = block0.Write(h0[:])
	_ = wire.WriteCompactSize(&block0, 1)
	_, _ = block0.Write(txb)

	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), h0[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	fx, err := store.OpenBlockFilterIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	body := block0.Bytes()
	if err := raw.Put(id0, body); err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(id0, body); err != nil {
		t.Fatal(err)
	}
	if err := IndexBasicBlockFilter(fx, id0, body, j, raw, ix); err != nil {
		t.Fatal(err)
	}
	filterPath := filepath.Join(fx.Dir(), hex.EncodeToString(id0[:])+".dat")
	if err := os.Remove(filterPath); err != nil {
		t.Fatal(err)
	}
	if fx.Has(id0) {
		t.Fatal("filter should be gone after wipe")
	}

	paths := &DataPaths{ChainDataDir: dir}
	res, code, msg := execReindexBlockFilters(paths, j, raw, ix, fx)
	if code != 0 {
		t.Fatalf("reindexblockfilters: code=%d msg=%q", code, msg)
	}
	m := res.(map[string]interface{})
	if v, _ := m["blocks_indexed"].(int); v < 1 {
		t.Fatalf("blocks_indexed=%v", m["blocks_indexed"])
	}
	if !fx.Has(id0) {
		t.Fatal("expected filter after reindex")
	}
	height, _ := json.Marshal(float64(0))
	out, code, msg := execGetBlockFilter(j, raw, ix, fx, []json.RawMessage{height})
	if code != 0 {
		t.Fatalf("getblockfilter: %d %s", code, msg)
	}
	if out.(map[string]interface{})["filter"] == "" {
		t.Fatal("empty filter after reindex")
	}
}

func TestExecReindexBlockFiltersIdempotentSecondPass(t *testing.T) {
	txb := minimalCoinbaseTxBytes(t)
	cbTx, err := wire.DeserializeTx(txb)
	if err != nil {
		t.Fatal(err)
	}
	mr0 := wire.BlockMerkleRoot([]*wire.Tx{cbTx})
	hdr0 := primitives.BlockHeader{Version: 1, MerkleRoot: mr0, Timestamp: 1700000000, Bits: 0x1e0ffff0, Nonce: 42}
	h0 := hdr0.EncodeWire80()
	id0 := pow.BlockHashLE(h0[:])
	var block0 bytes.Buffer
	_, _ = block0.Write(h0[:])
	_ = wire.WriteCompactSize(&block0, 1)
	_, _ = block0.Write(txb)

	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), h0[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	fx, err := store.OpenBlockFilterIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	body := block0.Bytes()
	if err := raw.Put(id0, body); err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(id0, body); err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{ChainDataDir: dir}
	res, code, msg := execReindexBlockFilters(paths, j, raw, ix, fx)
	if code != 0 {
		t.Fatalf("first pass: code=%d msg=%q", code, msg)
	}
	first := res.(map[string]interface{})["blocks_indexed"].(int)
	res2, code, msg := execReindexBlockFilters(paths, j, raw, ix, fx)
	if code != 0 {
		t.Fatalf("second pass: code=%d msg=%q", code, msg)
	}
	second := res2.(map[string]interface{})["blocks_indexed"].(int)
	if first < 1 || second < 1 {
		t.Fatalf("blocks_indexed first=%d second=%d", first, second)
	}
	if !fx.Has(id0) {
		t.Fatal("filter missing after idempotent reindex")
	}
}
