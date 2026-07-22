// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func TestHeaderCheckpointGenesisMainnet(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	SetHeaderCheckpointsEnabled(true)
	if err := checkHeaderCheckpoint(p.Net, 0, g80[:]); err != nil {
		t.Fatal(err)
	}
	g80[76] ^= 1
	if err := checkHeaderCheckpoint(p.Net, 0, g80[:]); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestValidateHeadersRejectsTimeRegression(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "h.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	prevHash := pow.BlockHashLE(g80[:])
	h1 := append([]byte(nil), g80[:]...)
	copy(h1[4:36], prevHash[:])
	h1[76] ^= 0x11
	binaryPutTime(h1, 2000)
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	h1Hash := pow.BlockHashLE(h1)
	h2 := append([]byte(nil), h1...)
	copy(h2[4:36], h1Hash[:])
	h2[76] ^= 0x22
	binaryPutTime(h2, 1000)
	decoded := []wire.DecodedHeader{{Header80: h2}}
	if err := ValidateHeaders(j, p, decoded, 9_000_000); err == nil {
		t.Fatal("expected nTime regression error")
	}
}

func binaryPutTime(h []byte, t uint32) {
	h[68] = byte(t)
	h[69] = byte(t >> 8)
	h[70] = byte(t >> 16)
	h[71] = byte(t >> 24)
}
