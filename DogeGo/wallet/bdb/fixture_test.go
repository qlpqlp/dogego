// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package bdb

import (
	"path/filepath"
	"testing"
)

func TestWriteFixtureWalletRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallet.dat")
	kv := map[string][]byte{
		"alpha": []byte("one"),
		"beta":  []byte("two"),
	}
	if err := FixtureRoundTrip(path, kv); err != nil {
		t.Fatal(err)
	}
	if !IsBDBFile(path) {
		t.Fatal("expected bdb magic")
	}
}
