// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestValidateStoredHeaders_rebootTestnetRequiresPoW(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	if p.RelaxedPoW {
		t.Fatal("reboot testnet should use real scrypt PoW validation")
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(dir+"/headers.bin", g80[:])
	if err != nil {
		t.Fatal(err)
	}
	h1 := make([]byte, 80)
	genHash := pow.BlockHashLE(g80[:])
	copy(h1[0:4], g80[0:4])
	copy(h1[4:36], genHash[:])
	copy(h1[36:68], g80[36:68])
	binary.LittleEndian.PutUint32(h1[68:72], binary.LittleEndian.Uint32(g80[68:72])+1)
	binary.LittleEndian.PutUint32(h1[72:76], binary.LittleEndian.Uint32(g80[72:76]))
	binary.LittleEndian.PutUint32(h1[76:80], binary.LittleEndian.Uint32(g80[76:80])+1)
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateStoredHeaders(j, nil, p, 0, 1, time.Now().Unix()); err == nil {
		t.Fatal("expected scrypt PoW failure on invalid height-1 header")
	}
}
