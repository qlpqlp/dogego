// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"testing"
)

func TestResolveIconBytesWithoutManager(t *testing.T) {
	for _, id := range []string{"dogego.zkl2", "dogego.doginals", "dogego.bbpow", "dogego.radiodoge", "example.go", "example.wasm"} {
		b, err := ResolveIconBytes(nil, id)
		if err != nil || len(b) < 8 {
			t.Fatalf("%s icon without mgr: err=%v len=%d", id, err, len(b))
		}
	}
}

func TestManagerIconBytesBuiltin(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, "testnet", nil)
	for _, id := range []string{"dogego.zkl2", "dogego.doginals", "dogego.bbpow", "dogego.radiodoge", "example.go", "example.wasm"} {
		b, err := m.IconBytes(id)
		if err != nil || len(b) < 8 {
			t.Fatalf("%s icon: err=%v len=%d", id, err, len(b))
		}
		if b[0] != 0x89 || b[1] != 'P' || b[2] != 'N' || b[3] != 'G' {
			t.Fatalf("%s not png magic: %x", id, b[:4])
		}
	}
}

func TestValidateIconRelRejectsAbsolute(t *testing.T) {
	if err := ValidateIconRel("/etc/passwd.png"); err == nil {
		t.Fatal("expected reject absolute icon path")
	}
}
