// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package version

import (
	"strings"
	"testing"
)

func TestParseChecksumSidecar(t *testing.T) {
	hash := "a" + strings.Repeat("b", 63)
	raw := hash + "  dogego-windows-amd64.exe\n"
	got, err := ParseChecksumSidecar([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got != hash {
		t.Fatalf("got %q want %q", got, hash)
	}
}
