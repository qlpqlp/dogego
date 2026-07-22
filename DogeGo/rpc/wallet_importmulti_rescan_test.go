// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestImportMultiRescan(t *testing.T) {
	var synced, scanned bool
	paths := &DataPaths{
		WalletDefaultAddress: func() string { return "TAddr" },
		WalletImportWatch:    func([]byte) error { return nil },
		SyncUtxo:             func() error { synced = true; return nil },
		WalletRescanBlocks:   func(int64) error { scanned = true; return nil },
	}
	req := `{"scriptPubKey":"76a914` + strings.Repeat("00", 20) + `88ac"}`
	out, code, msg := execImportMultiWallet("test", paths, nil, nil, []json.RawMessage{
		json.RawMessage(`[` + req + `]`),
		json.RawMessage(`{"rescan":true}`),
	})
	if code != 0 {
		t.Fatalf("importmulti: %d %s", code, msg)
	}
	rows, ok := out.([]map[string]interface{})
	if !ok || len(rows) != 1 || rows[0]["success"] != true {
		t.Fatalf("result %#v", out)
	}
	if !synced || !scanned {
		t.Fatalf("sync=%v scan=%v", synced, scanned)
	}
}
