// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package upnp

import "testing"

func TestShouldMap(t *testing.T) {
	if ShouldMap(ModeNever, true) {
		t.Fatal("never")
	}
	if !ShouldMap(ModeEnable, false) {
		t.Fatal("enable without listen")
	}
	if !ShouldMap(ModeAuto, true) {
		t.Fatal("auto with listen")
	}
	if ShouldMap(ModeAuto, false) {
		t.Fatal("auto without listen")
	}
}

func TestParseMode(t *testing.T) {
	if ParseMode("") != ModeAuto {
		t.Fatal("empty")
	}
	if ParseMode("disable") != ModeNever {
		t.Fatal("disable")
	}
	if ParseMode("enable") != ModeEnable {
		t.Fatal("enable")
	}
}
