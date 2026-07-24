// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package catalog

import "testing"

func TestReadIconBytesEmbedded(t *testing.T) {
	for _, dir := range []string{"zkl2", "doginals", "bbpow", "radiodoge", "example-go", "example-wasm"} {
		b, err := ReadIconBytes(dir, "icon.png")
		if err != nil || len(b) < 8 {
			t.Fatalf("%s icon: err=%v len=%d", dir, err, len(b))
		}
		if b[0] != 0x89 || b[1] != 'P' {
			t.Fatalf("%s not png", dir)
		}
	}
}
