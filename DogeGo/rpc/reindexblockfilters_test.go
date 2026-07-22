// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"path/filepath"
	"strings"
	"testing"

	"dogego/store"
)

func TestExecReindexBlockFiltersRequiresTxIndex(t *testing.T) {
	rawBlk, _ := store.TestMinimalBlock()
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), rawBlk[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{ChainDataDir: dir}
	_, code, msg := execReindexBlockFilters(paths, j, raw, nil, nil)
	if code != -1 || !strings.Contains(msg, "tx index") {
		t.Fatalf("want tx index error, got %d %q", code, msg)
	}
}
