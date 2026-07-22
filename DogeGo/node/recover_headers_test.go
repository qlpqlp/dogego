// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestRecoverHeaderJournalNilJournal(t *testing.T) {
	_, err := RecoverHeaderJournal(nil, nil, chain.Params{}, nil)
	if err == nil {
		t.Fatal("expected error for nil journal")
	}
}

func TestRecoverHeaderJournalGenesisOnly(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(dir+"/h.bin", g80[:])
	if err != nil {
		t.Fatal(err)
	}
	_, err = RecoverHeaderJournal(j, nil, p, NewBlockStoreCtx(j, nil, p, nil, nil, nil))
	if err == nil {
		t.Fatal("expected no-change error at genesis-only tip")
	}
}
