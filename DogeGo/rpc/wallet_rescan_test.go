// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"

	"dogego/store"
)

func TestRescanWalletSyncUtxo(t *testing.T) {
	var synced bool
	paths := &DataPaths{
		WalletDefaultAddress: func() string { return "DAddr" },
		SyncUtxo: func() error {
			synced = true
			return nil
		},
	}
	_, code, msg := execRescanWallet(paths, &memJournal{}, nil, nil)
	if code != 0 {
		t.Fatalf("rescan: %s", msg)
	}
	if !synced {
		t.Fatal("expected SyncUtxo")
	}
}

func TestRescanWalletSkipsSyncUtxoWhenCaughtUp(t *testing.T) {
	var synced bool
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(50)
	paths := &DataPaths{
		WalletDefaultAddress: func() string { return "DAddr" },
		Utxo:                 utxo,
		SyncUtxo: func() error {
			synced = true
			return nil
		},
	}
	j := &memJournal{tip: 50, count: 51, hdrs: make([][]byte, 51)}
	for i := range j.hdrs {
		j.hdrs[i] = make([]byte, 80)
	}
	_, code, msg := execRescanWallet(paths, j, nil, nil)
	if code != 0 {
		t.Fatalf("rescan: %s", msg)
	}
	if synced {
		t.Fatal("expected SyncUtxo skipped when UTXO tip matches journal")
	}
}

func TestRescanWalletHeightRange(t *testing.T) {
	j := &memJournal{tip: 10, count: 11, hdrs: make([][]byte, 11)}
	for i := range j.hdrs {
		j.hdrs[i] = make([]byte, 80)
	}
	paths := &DataPaths{WalletDefaultAddress: func() string { return "DAddr"}}
	h, _ := json.Marshal(int64(99))
	_, code, msg := execRescanWallet(paths, j, nil, []json.RawMessage{h})
	if code != -8 || msg == "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}
