// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package catalog

import "testing"

func TestDiscoverPackagesIncludesOfficial(t *testing.T) {
	pkgs := Packages()
	want := []string{"dogego.zkl2", "dogego.doginals", "dogego.bbpow", "dogego.radiodoge", "example.go", "example.wasm"}
	for _, id := range want {
		p, ok := pkgs[id]
		if !ok || p.Dir == "" {
			t.Fatalf("missing package %s (got %#v)", id, pkgs)
		}
		dir, ok := PackageDir(id)
		if !ok || dir != p.Dir {
			t.Fatalf("PackageDir(%s)=%q ok=%v", id, dir, ok)
		}
		if PackageIconRel(id) == "" {
			t.Fatalf("PackageIconRel(%s) empty", id)
		}
	}
}
