// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSanitizeZipEntryRelStripsPowershellParent(t *testing.T) {
	got, err := sanitizeZipEntryRel("../icon.png")
	if err != nil {
		t.Fatal(err)
	}
	if got != "icon.png" {
		t.Fatalf("got %q", got)
	}
	got, err = sanitizeZipEntryRel(`..\icon.png`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "icon.png" {
		t.Fatalf("got %q", got)
	}
	got, err = sanitizeZipEntryRel("../../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.FromSlash("etc/passwd")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHostNativeExecutableMagic(t *testing.T) {
	dir := t.TempDir()
	pe := filepath.Join(dir, "pe.bin")
	elf := filepath.Join(dir, "elf.bin")
	if err := os.WriteFile(pe, []byte("MZ........"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(elf, []byte{0x7f, 'E', 'L', 'F', 0, 0, 0, 0}, 0o644); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		if !hostNativeExecutable(pe) {
			t.Fatal("expected PE native on windows")
		}
		if hostNativeExecutable(elf) {
			t.Fatal("ELF must not be native on windows")
		}
	} else if runtime.GOOS != "darwin" {
		if !hostNativeExecutable(elf) {
			t.Fatal("expected ELF native")
		}
		if hostNativeExecutable(pe) {
			t.Fatal("PE must not be native on unix")
		}
	}
}
