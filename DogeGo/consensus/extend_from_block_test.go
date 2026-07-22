// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"path/filepath"
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestExtendHeadersFromParentHeightRejectsWrongParent(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), gen[:])
	if err != nil {
		t.Fatal(err)
	}
	genHash := pow.BlockHashLE(gen[:])
	h1 := make([]byte, 80)
	copy(h1[4:36], genHash[:])
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	h2 := make([]byte, 80)
	copy(h2[4:36], genHash[:])
	var hdr primitives.BlockHeader
	if err := hdr.DecodeWire80(h2); err != nil {
		t.Fatal(err)
	}
	pb := &wire.ParsedBlock{Header: hdr}
	_, err = ExtendHeadersFromParentHeight(j, nil, p, pb, 0, time.Now().Unix())
	if err == nil {
		t.Fatal("expected error when parent is not journal tip")
	}
}

func binaryLE32(b []byte, off int, v uint32) {
	b[off] = byte(v)
	b[off+1] = byte(v >> 8)
	b[off+2] = byte(v >> 16)
	b[off+3] = byte(v >> 24)
}
