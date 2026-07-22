// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package netfw

import "testing"

func TestParseMode(t *testing.T) {
	tests := []struct {
		in   string
		want Mode
	}{
		{"", ModeAuto},
		{"auto", ModeAuto},
		{"always", ModeAlways},
		{"never", ModeNever},
		{"off", ModeNever},
	}
	for _, tc := range tests {
		if got := ParseMode(tc.in); got != tc.want {
			t.Fatalf("ParseMode(%q) = %v want %v", tc.in, got, tc.want)
		}
	}
}
