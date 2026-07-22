// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
)

func TestHeaderJournalOpenCountTip(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "headers.bin")
	j, err := OpenHeaderJournal(path, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	n, err := j.Count()
	if err != nil || n != 1 {
		t.Fatalf("Count: n=%d err=%v", n, err)
	}
	tip, err := j.TipHeight()
	if err != nil || tip != 0 {
		t.Fatalf("TipHeight: %d err=%v", tip, err)
	}
	gen, err := j.GenesisHashHex()
	if err != nil {
		t.Fatal(err)
	}
	if gen != p.GenesisBlockHashHex {
		t.Fatalf("genesis hex: got %s want %s", gen, p.GenesisBlockHashHex)
	}
	best, err := j.BestBlockHashHex()
	if err != nil {
		t.Fatal(err)
	}
	if best != gen {
		t.Fatalf("best at height 0 should equal genesis: %s vs %s", best, gen)
	}
}

func TestHeaderJournalAppend(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "h.bin")
	j, err := OpenHeaderJournal(path, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	h2 := append([]byte(nil), g80[:]...)
	h2[76] ^= 0x01 // tweak nonce tail (not necessarily valid PoW; store does not check)
	if err := j.AppendHeaders([][]byte{h2}); err != nil {
		t.Fatal(err)
	}
	n, err := j.Count()
	if err != nil || n != 2 {
		t.Fatalf("after append Count: n=%d err=%v", n, err)
	}
	tip, err := j.TipHeight()
	if err != nil || tip != 1 {
		t.Fatalf("TipHeight: %d err=%v", tip, err)
	}
	all, err := j.ReadAll()
	if err != nil || len(all) != 2 {
		t.Fatalf("ReadAll: len=%d err=%v", len(all), err)
	}
}

func TestReadHeaderAt(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "x.bin")
	j, err := OpenHeaderJournal(path, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	h0, err := j.ReadHeaderAt(0)
	if err != nil || len(h0) != 80 {
		t.Fatalf("ReadHeaderAt 0: %v len %d", err, len(h0))
	}
	if !bytes.Equal(h0, g80[:]) {
		t.Fatal("genesis mismatch")
	}
}

func TestHeaderJournalTruncateToHeight(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "t.bin")
	j, err := OpenHeaderJournal(path, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	h2 := append([]byte(nil), g80[:]...)
	h2[76] ^= 0x02
	if err := j.AppendHeaders([][]byte{h2}); err != nil {
		t.Fatal(err)
	}
	if err := j.TruncateToHeight(0); err != nil {
		t.Fatal(err)
	}
	n, _ := j.Count()
	if n != 1 {
		t.Fatalf("after truncate want 1 header got %d", n)
	}
	hash0, _ := j.LastTipHash()
	if hash0 != pow.BlockHashLE(g80[:]) {
		t.Fatal("tip should be genesis after truncate")
	}
}

func TestHeaderJournalHeightByDisplayHash(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	want := pow.BlockHashHex(g80[:])
	path := filepath.Join(dir, "z.bin")
	j, err := OpenHeaderJournal(path, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	h, err := j.HeightByDisplayHash(want)
	if err != nil || h != 0 {
		t.Fatalf("HeightByDisplayHash: h=%d err=%v", h, err)
	}
	if _, err := j.HeightByDisplayHash("0000000000000000000000000000000000000000000000000000000000000001"); err == nil {
		t.Fatal("expected error for unknown hash")
	}
}

func TestBuildBlockLocator(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "loc.bin")
	j, err := OpenHeaderJournal(path, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	loc1, err := j.BuildBlockLocator(101)
	if err != nil || len(loc1) != 1 {
		t.Fatalf("loc1 len=%d err=%v", len(loc1), err)
	}
	want0 := pow.BlockHashLE(g80[:])
	if loc1[0] != want0 {
		t.Fatal("single locator mismatch")
	}
	h2 := append([]byte(nil), g80[:]...)
	h2[76] ^= 0x02
	if err := j.AppendHeaders([][]byte{h2}); err != nil {
		t.Fatal(err)
	}
	loc2, err := j.BuildBlockLocator(101)
	if err != nil || len(loc2) < 2 {
		t.Fatalf("loc2 len=%d err=%v", len(loc2), err)
	}
	tip := pow.BlockHashLE(h2)
	if loc2[0] != tip {
		t.Fatalf("first locator want tip")
	}
	if loc2[1] != want0 {
		t.Fatalf("second locator want genesis")
	}
	locFork, err := j.BuildBlockLocatorFromHeight(0, 101)
	if err != nil || len(locFork) != 1 || locFork[0] != want0 {
		t.Fatalf("fork locator from genesis: %v err=%v", locFork, err)
	}
	locMid, err := j.BuildBlockLocatorFromHeight(1, 101)
	if err != nil || len(locMid) < 2 || locMid[0] != tip {
		t.Fatalf("fork locator from height 1: %v err=%v", locMid, err)
	}
}

func TestReadHeadersThrough(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "through.bin")
	j, err := OpenHeaderJournal(path, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	h2 := append([]byte(nil), g80[:]...)
	h2[76] ^= 0x02
	if err := j.AppendHeaders([][]byte{h2}); err != nil {
		t.Fatal(err)
	}
	raw, err := j.ReadHeadersThrough(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 160 {
		t.Fatalf("len %d", len(raw))
	}
	if !bytes.Equal(raw[:80], g80[:]) || !bytes.Equal(raw[80:], h2) {
		t.Fatal("snapshot mismatch")
	}
	if _, err := j.ReadHeadersThrough(2); err == nil {
		t.Fatal("expected error for beyond tip")
	}
}

func TestHeaderJournalRepairPartialTail(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "headers.bin")
	j, err := OpenHeaderJournal(path, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	h2 := append([]byte(nil), g80[:]...)
	h2[76] ^= 0x03
	if err := j.AppendHeaders([][]byte{h2}); err != nil {
		t.Fatal(err)
	}
	// Simulate kill mid-append: torn 40-byte tail.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(make([]byte, 40)); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	j2, err := OpenHeaderJournal(path, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	tip, err := j2.TipHeight()
	if err != nil || tip != 1 {
		t.Fatalf("after repair TipHeight: %d err=%v", tip, err)
	}
}
