// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package corewallet

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProbeWalletDatNotBDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := ProbeWalletDat(path, 0x9e)
	if err != nil {
		t.Fatal(err)
	}
	if p.IsBDB || p.CanImport {
		t.Fatalf("probe %#v", p)
	}
}
