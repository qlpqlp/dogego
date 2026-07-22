// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// TestExecReindexTxRebuildsIndex verifies operator reindextx over a small stored chain.
func TestExecReindexTxRebuildsIndex(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	rs.EnableTxIndexing(ix, true)
	prevID := pow.BlockHashLE(genesisRaw[:80])
	if err := rs.Put(prevID, genesisRaw); err != nil {
		t.Fatal(err)
	}
	for h := int64(1); h <= 3; h++ {
		prev, _ := j.ReadHeaderAt(h - 1)
		hdr := append([]byte(nil), prev...)
		copy(hdr[4:36], prevID[:])
		hdr[76] ^= byte(h)
		body := store.MakeTestBlockRaw(t, hdr)
		stored := append([]byte(nil), body[:80]...)
		id := pow.BlockHashLE(stored)
		if err := j.AppendHeaders([][]byte{stored}); err != nil {
			t.Fatal(err)
		}
		if err := rs.Put(id, body); err != nil {
			t.Fatal(err)
		}
		prevID = id
	}

	paths := &DataPaths{ChainDataDir: dir}
	clear, _ := json.Marshal(true)
	res, code, msg := execReindexTx(paths, []json.RawMessage{clear})
	if code != 0 {
		t.Fatalf("reindextx: code=%d msg=%q", code, msg)
	}
	m := res.(map[string]interface{})
	if v, _ := m["blocks_indexed"].(int); v < 1 {
		t.Fatalf("blocks_indexed=%v", m["blocks_indexed"])
	}
	txN, _, err := ix.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if txN < 1 {
		t.Fatalf("tx index files after reindex: %d", txN)
	}
}

func TestExecReindexTxIdempotentSecondPass(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	prevID := pow.BlockHashLE(genesisRaw[:80])
	if err := rs.Put(prevID, genesisRaw); err != nil {
		t.Fatal(err)
	}
	for h := int64(1); h <= 2; h++ {
		prev, _ := j.ReadHeaderAt(h - 1)
		hdr := append([]byte(nil), prev...)
		copy(hdr[4:36], prevID[:])
		hdr[76] ^= byte(h)
		body := store.MakeTestBlockRaw(t, hdr)
		id := pow.BlockHashLE(body[:80])
		if err := j.AppendHeaders([][]byte{body[:80]}); err != nil {
			t.Fatal(err)
		}
		if err := rs.Put(id, body); err != nil {
			t.Fatal(err)
		}
		prevID = id
	}
	paths := &DataPaths{ChainDataDir: dir}
	clear, _ := json.Marshal(true)
	if _, code, msg := execReindexTx(paths, []json.RawMessage{clear}); code != 0 {
		t.Fatalf("first reindextx: code=%d msg=%q", code, msg)
	}
	res, code, msg := execReindexTx(paths, nil)
	if code != 0 {
		t.Fatalf("second reindextx: code=%d msg=%q", code, msg)
	}
	m := res.(map[string]interface{})
	if v, _ := m["blocks_indexed"].(int); v < 1 {
		t.Fatalf("blocks_indexed=%v", m["blocks_indexed"])
	}
}
