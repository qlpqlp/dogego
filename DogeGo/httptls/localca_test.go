// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package httptls

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureLocalMaterial_regeneratesAfterCADelete(t *testing.T) {
	dir := t.TempDir()
	mat1, err := EnsureLocalMaterial(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !mat1.CAGenerated {
		t.Fatal("first run should generate CA")
	}
	fp1, err := CACertSHA1Hex(mat1.CACertPath)
	if err != nil {
		t.Fatal(err)
	}

	mat2, err := EnsureLocalMaterial(dir)
	if err != nil {
		t.Fatal(err)
	}
	if mat2.CAGenerated {
		t.Fatal("second run should reuse CA")
	}
	fp2, err := CACertSHA1Hex(mat2.CACertPath)
	if err != nil {
		t.Fatal(err)
	}
	if fp1 != fp2 {
		t.Fatalf("fingerprints differ on reuse: %s vs %s", fp1, fp2)
	}

	if err := os.Remove(mat1.CACertPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(mat1.CAKeyPath); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(mat1.Dir, "webui.crt")
	if err := os.WriteFile(leaf, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	mat3, err := EnsureLocalMaterial(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !mat3.CAGenerated {
		t.Fatal("expected new CA after delete")
	}
	if _, err := os.Stat(leaf); err == nil {
		t.Fatal("stale leaf should be removed when CA is regenerated")
	}
	fp3, err := CACertSHA1Hex(mat3.CACertPath)
	if err != nil {
		t.Fatal(err)
	}
	if fp3 == fp1 {
		t.Fatal("expected new CA fingerprint after delete")
	}
}
