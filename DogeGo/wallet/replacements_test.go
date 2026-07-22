// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
)

func TestRecordTxReplacementPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	oldID := strings.Repeat("a", 64)
	newID := strings.Repeat("b", 64)
	if err := w.RecordTxReplacement(oldID, newID); err != nil {
		t.Fatal(err)
	}
	if got := w.ConflictsForTx(oldID); len(got) != 1 || got[0] != newID {
		t.Fatalf("old conflicts %v", got)
	}
	if got := w.ConflictsForTx(newID); len(got) != 1 || got[0] != oldID {
		t.Fatalf("new conflicts %v", got)
	}
	w2, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if got := w2.ConflictsForTx(oldID); len(got) != 1 || got[0] != newID {
		t.Fatalf("reload conflicts %v", got)
	}
}
