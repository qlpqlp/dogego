// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"testing"

	"dogego/pow"
	"dogego/store"
)

func TestStoredHeaderRangeNeedsAux(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(dir+"/h.bin", g80[:])
	if err != nil {
		t.Fatal(err)
	}
	aux80 := append([]byte(nil), g80[:]...)
	ver := binary.LittleEndian.Uint32(aux80[0:4]) | (1 << 8)
	binary.LittleEndian.PutUint32(aux80[0:4], ver)
	if err := j.AppendHeaders([][]byte{aux80}); err != nil {
		t.Fatal(err)
	}
	needs, err := StoredHeaderRangeNeedsAux(j, 1, 1)
	if err != nil || !needs {
		t.Fatalf("needs=%v err=%v", needs, err)
	}
	needs, err = StoredHeaderRangeNeedsAux(j, 0, 0)
	if err != nil || needs {
		t.Fatalf("genesis needs=%v err=%v", needs, err)
	}
}
