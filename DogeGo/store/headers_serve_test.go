// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/wire"
)

func TestFindLocatorForkHeight_andHeadersAfterFork(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	h2 := append([]byte(nil), g80[:]...)
	h2[76] ^= 1
	if err := j.AppendHeaders([][]byte{h2}); err != nil {
		t.Fatal(err)
	}
	genLE := pow.BlockHashLE(g80[:])
	fork, err := FindLocatorForkHeight(j, [][32]byte{genLE})
	if err != nil || fork != 0 {
		t.Fatalf("fork at genesis: %d err %v", fork, err)
	}
	headers, err := HeadersAfterFork(j, nil, fork, [32]byte{}, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 1 {
		t.Fatalf("want 1 header after genesis, got %d", len(headers))
	}
	pl, err := wire.EncodeHeadersPayload(headers)
	if err != nil {
		t.Fatal(err)
	}
	got, err := wire.DecodeHeadersPayload(pl)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || string(got[0].Header80) != string(h2) {
		t.Fatal("round-trip mismatch")
	}
}

func TestHeadersAfterForkHashStop(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	prevID := pow.BlockHashLE(g80[:])
	for i := byte(1); i <= 3; i++ {
		h := append([]byte(nil), g80[:]...)
		copy(h[4:36], prevID[:])
		h[76] ^= i
		if err := j.AppendHeaders([][]byte{h}); err != nil {
			t.Fatal(err)
		}
		prevID = pow.BlockHashLE(h)
	}
	h2, _ := j.ReadHeaderAt(2)
	stop := pow.BlockHashLE(h2)
	headers, err := HeadersAfterFork(j, nil, 0, stop, MaxHeadersPerMessage)
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 1 {
		t.Fatalf("hashStop at height 2: got %d headers want 1 (height 1 only)", len(headers))
	}
	want1, _ := j.ReadHeaderAt(1)
	if string(headers[0].Header80) != string(want1) {
		t.Fatal("header should be height 1")
	}
}

func TestHeadersAfterForkMaxCap(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	prevID := pow.BlockHashLE(g80[:])
	for i := byte(1); i <= 5; i++ {
		h := append([]byte(nil), g80[:]...)
		copy(h[4:36], prevID[:])
		h[76] ^= i
		if err := j.AppendHeaders([][]byte{h}); err != nil {
			t.Fatal(err)
		}
		prevID = pow.BlockHashLE(h)
	}
	headers, err := HeadersAfterFork(j, nil, 0, [32]byte{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 2 {
		t.Fatalf("max cap 2: got %d", len(headers))
	}
}
