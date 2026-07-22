// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"bytes"
	"testing"

	"dogego/pow"
	"dogego/wire"
)

func TestBlockRecordZstdRoundTrip(t *testing.T) {
	var hash [32]byte
	hash[0] = 0xab
	raw := bytes.Repeat([]byte{0x01}, 4000)
	rec, err := encodeBlockRecord(hash, raw, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec) >= len(raw)+blockRecordHeaderLen {
		t.Fatalf("expected compression, rec len %d", len(rec))
	}
	got, err := decodeBlockRecord(rec, hash)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatal("payload mismatch after zstd round trip")
	}
}

func TestRawBlockStoreBundledPutGet(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: true})
	if err != nil {
		t.Fatal(err)
	}
	payload, hash := TestMinimalBlock()
	if err := raw.Put(hash, payload); err != nil {
		t.Fatal(err)
	}
	got, err := raw.Get(hash)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("bundled get mismatch")
	}
	got0, err := raw.GetByContiguousHeight(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got0, payload) {
		t.Fatal("bundled contiguous height 0 mismatch")
	}
	tip, err := raw.ProbeBundledContiguousTip()
	if err != nil || tip != 0 {
		t.Fatalf("probe tip=%d err=%v want 0", tip, err)
	}
	if raw.StorageOpts().Layout != BlockLayoutBundled {
		t.Fatalf("opts %#v", raw.StorageOpts())
	}
}

func TestRawBlockStoreBundledFourSequential(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	payload, id := TestMinimalBlock()
	if err := raw.Put(id, payload); err != nil {
		t.Fatal(err)
	}
	prev := append([]byte(nil), payload[:80]...)
	for i := 1; i <= 3; i++ {
		h80 := append([]byte(nil), prev...)
		ph := pow.BlockHashLE(prev)
		copy(h80[4:36], ph[:])
		h80[76] ^= byte(i)
		next := MakeTestBlockRaw(t, h80)
		id := pow.BlockHashLE(next[:80])
		if err := raw.Put(id, next); err != nil {
			t.Fatal(err)
		}
		prev = append([]byte(nil), next[:80]...)
	}
	n, err := raw.Count()
	if err != nil || n != 4 {
		t.Fatalf("count=%d err=%v want 4", n, err)
	}
	tip, err := raw.ProbeBundledContiguousTip()
	if err != nil || tip != 3 {
		t.Fatalf("probe tip=%d err=%v want 3", tip, err)
	}
	for h := int64(0); h <= 3; h++ {
		if _, err := raw.GetByContiguousHeight(h); err != nil {
			t.Fatalf("height %d: %v", h, err)
		}
	}
}

func TestRawBlockStorePerFileZstd(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutPerFile, Zstd: true})
	if err != nil {
		t.Fatal(err)
	}
	payload, hash := TestMinimalBlock()
	if err := raw.Put(hash, payload); err != nil {
		t.Fatal(err)
	}
	got, err := raw.Get(hash)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("per-file zstd get mismatch")
	}
}

func TestTxIndexOffsetOnly(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := OpenTxIndexWithOpts(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	payload, hash := TestMinimalBlock()
	if err := raw.Put(hash, payload); err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(hash, payload); err != nil {
		t.Fatal(err)
	}
	var txid string
	err = wire.ForEachBlockTx(payload, func(_ uint32, tx *wire.Tx) error {
		txid = txidRPCFileName(tx.TxHash())
		return nil
	})
	if err != nil || txid == "" {
		t.Fatalf("txid: %v", err)
	}
	hit, err := ix.LookupHit(txid)
	if err != nil {
		t.Fatal(err)
	}
	if len(hit.TxRaw) != 0 {
		t.Fatal("expected offset-only index entry")
	}
	loaded, err := LoadIndexedTx(ix, raw, txid)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("nil tx")
	}
}
