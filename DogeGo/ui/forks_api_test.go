// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package ui

import "testing"

func TestClassifyPeerTip(t *testing.T) {
	st, _ := classifyPeerTip(100, "aaa", 100, "bbb")
	if st != "diverged" {
		t.Fatalf("got %s", st)
	}
	st, _ = classifyPeerTip(100, "aaa", 100, "aaa")
	if st != "aligned" {
		t.Fatalf("got %s", st)
	}
	st, _ = classifyPeerTip(100, "aaa", 120, "ccc")
	if st != "ahead" {
		t.Fatalf("got %s", st)
	}
	st, _ = classifyPeerTip(100, "aaa", 80, "")
	if st != "behind" {
		t.Fatalf("got %s", st)
	}
}
