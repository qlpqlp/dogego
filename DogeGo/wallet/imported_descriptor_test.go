// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"path/filepath"
	"testing"

	"dogego/chain"
)

func TestImportedDescriptorPersist(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	w, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	desc := "pkh(DTestImportDesc1111111111111111111)"
	if err := w.AddImportedDescriptor(desc, 1700000000, true, false); err != nil {
		t.Fatal(err)
	}
	w2, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	rows := w2.ListDescriptors(p.PubkeyHashAddrID, p.ScriptHashAddrID)
	var found bool
	for _, r := range rows {
		if r.Desc == desc && r.Internal && r.Timestamp == 1700000000 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("listdescriptors missing import metadata: %+v", rows)
	}
}
