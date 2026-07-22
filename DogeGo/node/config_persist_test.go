// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dogego/config"
)

func TestPersistMaxOutbound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)
	eff := config.File{Network: "testnet"}
	if err := config.Save(path, eff); err != nil {
		t.Fatal(err)
	}
	if err := PersistMaxOutbound(path, &eff, 16); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f config.File
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if f.MaxOutbound != 16 {
		t.Fatalf("maxoutbound %d", f.MaxOutbound)
	}
}
