// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestCumulativeChainworkHeaderJournal(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "cj.bin")
	j, err := store.OpenHeaderJournal(path, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	bits := uint32(0x1e0ffff0)
	h2 := append([]byte(nil), g80[:]...)
	// Distinct header but same nBits for predictable double-work sum.
	binary.LittleEndian.PutUint32(h2[72:76], bits)
	h2[76] ^= 0x04
	if err := j.AppendHeaders([][]byte{h2}); err != nil {
		t.Fatal(err)
	}
	b0 := binary.LittleEndian.Uint32(g80[72:76])
	b1 := binary.LittleEndian.Uint32(h2[72:76])
	w0, err := pow.BlockProofFromBits(b0)
	if err != nil {
		t.Fatal(err)
	}
	w1, err := pow.BlockProofFromBits(b1)
	if err != nil {
		t.Fatal(err)
	}
	want := pow.ChainworkHex(new(big.Int).Add(w0, w1))
	got, err := cumulativeChainworkHex(j, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCumulativeChainworkConsistentWithRawSnapshot(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "snap.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := j.ReadHeadersThrough(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 80 || !bytes.Equal(raw, g80[:]) {
		t.Fatalf("snapshot len %d", len(raw))
	}
	got, err := cumulativeChainworkHex(j, 0)
	if err != nil {
		t.Fatal(err)
	}
	w, err := pow.BlockProofFromBits(binary.LittleEndian.Uint32(g80[72:76]))
	if err != nil {
		t.Fatal(err)
	}
	if got != pow.ChainworkHex(w) {
		t.Fatalf("chainwork %q proof hex %q", got, pow.ChainworkHex(w))
	}
}
