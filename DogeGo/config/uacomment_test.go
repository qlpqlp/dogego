// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import "testing"

func TestEffectiveUAComment(t *testing.T) {
	f := File{UAComment: "lab", UACommentTipAddress: "DKznsfbYgqKSg6FW1wHhAJxwHF6VSDkHGS"}
	got := f.EffectiveUAComment()
	want := "lab; DKznsfbYgqKSg6FW1wHhAJxwHF6VSDkHGS"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	fTip := File{UACommentTipAddress: "DAddr"}
	if fTip.EffectiveUAComment() != "DAddr" {
		t.Fatal("tip only")
	}
}

func TestValidateUACommentTipNetwork(t *testing.T) {
	if err := ValidateUACommentTip("DKznsfbYgqKSg6FW1wHhAJxwHF6VSDkHGS", "mainnet"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateUACommentTip("DKznsfbYgqKSg6FW1wHhAJxwHF6VSDkHGS", "testnet"); err == nil {
		t.Fatal("expected testnet mismatch")
	}
}
