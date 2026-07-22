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

func TestExecPruneBlockchainRemovesStoredBodies(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	genRaw := store.MakeTestBlockRaw(t, g80[:])
	genID := pow.BlockHashLE(genRaw[:80])
	if err := raw.Put(genID, genRaw); err != nil {
		t.Fatal(err)
	}
	prevID := genID
	for h := int64(1); h <= 3; h++ {
		prev, _ := j.ReadHeaderAt(h - 1)
		hdr := append([]byte(nil), prev...)
		copy(hdr[4:36], prevID[:])
		hdr[76] ^= byte(h)
		body := store.MakeTestBlockRaw(t, hdr)
		id := pow.BlockHashLE(body[:80])
		if err := j.AppendHeaders([][]byte{body[:80]}); err != nil {
			t.Fatal(err)
		}
		if err := raw.Put(id, body); err != nil {
			t.Fatal(err)
		}
		prevID = id
	}
	if !raw.Has(prevID) {
		t.Fatal("tip body missing before prune")
	}
	paths := &DataPaths{ContiguousRawHeight: func() int64 { return 3 }}
	res, code, msg := execPruneBlockchain(j, raw, nil, paths, []json.RawMessage{json.RawMessage(`2`)})
	if code != 0 {
		t.Fatalf("prune: %d %s", code, msg)
	}
	last, ok := res.(int64)
	if !ok || last != 1 {
		t.Fatalf("last height %#v", res)
	}
	for h := int64(0); h <= 1; h++ {
		prev, _ := j.ReadHeaderAt(h)
		id := pow.BlockHashLE(prev)
		if raw.Has(id) {
			t.Fatalf("height %d body should be pruned", h)
		}
	}
	if !raw.Has(prevID) {
		t.Fatal("tip body should remain after prune below 2")
	}
}

func TestExecPruneBlockchainIdempotentSecondCall(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	genRaw := store.MakeTestBlockRaw(t, g80[:])
	genID := pow.BlockHashLE(genRaw[:80])
	if err := raw.Put(genID, genRaw); err != nil {
		t.Fatal(err)
	}
	prevID := genID
	for h := int64(1); h <= 3; h++ {
		prev, _ := j.ReadHeaderAt(h - 1)
		hdr := append([]byte(nil), prev...)
		copy(hdr[4:36], prevID[:])
		hdr[76] ^= byte(h)
		body := store.MakeTestBlockRaw(t, hdr)
		id := pow.BlockHashLE(body[:80])
		if err := j.AppendHeaders([][]byte{body[:80]}); err != nil {
			t.Fatal(err)
		}
		if err := raw.Put(id, body); err != nil {
			t.Fatal(err)
		}
		prevID = id
	}
	paths := &DataPaths{ContiguousRawHeight: func() int64 { return 3 }}
	arg := []json.RawMessage{json.RawMessage(`2`)}
	res1, code, msg := execPruneBlockchain(j, raw, nil, paths, arg)
	if code != 0 {
		t.Fatalf("first prune: %d %s", code, msg)
	}
	if last, ok := res1.(int64); !ok || last != 1 {
		t.Fatalf("first last %#v", res1)
	}
	res2, code, msg := execPruneBlockchain(j, raw, nil, paths, arg)
	if code != 0 {
		t.Fatalf("second prune: %d %s", code, msg)
	}
	if n, ok := res2.(int64); !ok || n != 0 {
		t.Fatalf("second prune want 0 removed, got %#v", res2)
	}
}

func TestExecPruneBlockchainMarkerAndChainInfo(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	genRaw := store.MakeTestBlockRaw(t, g80[:])
	genID := pow.BlockHashLE(genRaw[:80])
	if err := raw.Put(genID, genRaw); err != nil {
		t.Fatal(err)
	}
	prevID := genID
	for h := int64(1); h <= 3; h++ {
		prev, _ := j.ReadHeaderAt(h - 1)
		hdr := append([]byte(nil), prev...)
		copy(hdr[4:36], prevID[:])
		hdr[76] ^= byte(h)
		body := store.MakeTestBlockRaw(t, hdr)
		id := pow.BlockHashLE(body[:80])
		if err := j.AppendHeaders([][]byte{body[:80]}); err != nil {
			t.Fatal(err)
		}
		if err := raw.Put(id, body); err != nil {
			t.Fatal(err)
		}
		prevID = id
	}
	paths := &DataPaths{
		ChainDataDir:        dir,
		ContiguousRawHeight: func() int64 { return 3 },
	}
	res, code, msg := execPruneBlockchain(j, raw, nil, paths, []json.RawMessage{json.RawMessage(`2`)})
	if code != 0 {
		t.Fatalf("prune: %d %s", code, msg)
	}
	last, ok := res.(int64)
	if !ok || last != 1 {
		t.Fatalf("last %#v want 1", res)
	}
	if !corePrunedFromSummary(paths) {
		t.Fatal("expected pruned=true after marker save")
	}
	if got := pruneHeightFromSummary(paths); got != int64(1) {
		t.Fatalf("prune_height %#v want 1", got)
	}
}

func TestExecVerifyChainAfterPrune(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	genRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	genID := pow.BlockHashLE(genRaw[:80])
	if err := raw.Put(genID, genRaw); err != nil {
		t.Fatal(err)
	}
	prevID := genID
	for h := int64(1); h <= 3; h++ {
		prev, _ := j.ReadHeaderAt(h - 1)
		hdr := append([]byte(nil), prev...)
		copy(hdr[4:36], prevID[:])
		hdr[76] ^= byte(h)
		body := store.MakeTestBlockRaw(t, hdr)
		id := pow.BlockHashLE(body[:80])
		if err := j.AppendHeaders([][]byte{body[:80]}); err != nil {
			t.Fatal(err)
		}
		if err := raw.Put(id, body); err != nil {
			t.Fatal(err)
		}
		prevID = id
	}
	paths := &DataPaths{
		ChainDataDir:        dir,
		ContiguousRawHeight: func() int64 { return 3 },
	}
	if _, code, msg := execPruneBlockchain(j, raw, nil, paths, []json.RawMessage{json.RawMessage(`2`)}); code != 0 {
		t.Fatalf("prune: %d %s", code, msg)
	}
	p1, _ := json.Marshal(2)
	p2, _ := json.Marshal(2)
	res, code, msg := execVerifyChain("testnet", j, nil, raw, nil, paths, nil, []json.RawMessage{p1, p2})
	if code != 0 || msg != "" {
		t.Fatalf("verifychain after prune: code=%d msg=%q", code, msg)
	}
	if res != true {
		t.Fatalf("verifychain result %#v", res)
	}
	id3, _ := j.ReadHeaderAt(3)
	_ = id3
	got, code, msg := execGetBlock(j, raw, nil, "testnet", paths, []json.RawMessage{json.RawMessage(`3`)})
	if code != 0 {
		t.Fatalf("getblock tip after prune: code=%d msg=%q", code, msg)
	}
	if got == nil {
		t.Fatal("nil getblock result")
	}
}
