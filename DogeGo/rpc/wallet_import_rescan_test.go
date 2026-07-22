// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"
)

func TestWalletRescanAfterImportHeight(t *testing.T) {
	var got int64
	paths := &DataPaths{
		SyncUtxo: func() error { return nil },
		WalletRescanBlocks: func(start int64) error {
			got = start
			return nil
		},
	}
	h, _ := json.Marshal(json.Number("42"))
	code, msg := walletRescanAfterImport(paths, nil, nil, []json.RawMessage{h}, 0, "importaddress")
	if code != 0 {
		t.Fatalf("rescan: %d %s", code, msg)
	}
	if got != 42 {
		t.Fatalf("start %d want 42", got)
	}
}
