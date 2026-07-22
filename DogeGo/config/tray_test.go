// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import "testing"

func TestTrayEnabled(t *testing.T) {
	on := true
	off := false
	fOn := File{Tray: &on}
	if !fOn.TrayEnabled(false) {
		t.Fatal("explicit true")
	}
	fOff := File{Tray: &off}
	if fOff.TrayEnabled(true) {
		t.Fatal("explicit false")
	}
	var empty File
	if !empty.TrayEnabled(true) {
		t.Fatal("nil uses default")
	}
}
