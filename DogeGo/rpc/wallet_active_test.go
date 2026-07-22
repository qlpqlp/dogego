// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestWalletActiveNil(t *testing.T) {
	if WalletActive(nil) {
		t.Fatal("nil paths")
	}
}

func TestWalletActiveImportWatch(t *testing.T) {
	paths := &DataPaths{
		WalletImportWatch: func(script []byte) error { return nil },
	}
	if !WalletActive(paths) {
		t.Fatal("expected active when import watch wired")
	}
}
