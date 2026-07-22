// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/wallet"
)

func TestGetBalancesEmptyWallet(t *testing.T) {
	res, code, msg := execGetBalances("test", nil, &memJournal{}, nil, nil, nil, nil)
	if code != 0 {
		t.Fatalf("getbalances: %s", msg)
	}
	m := res.(map[string]interface{})
	mine := m["mine"].(map[string]interface{})
	if mine["trusted"].(float64) != 0 {
		t.Fatalf("mine %#v", mine)
	}
}

func TestGetBalancesWithLabel(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, _ := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	_ = w.SetLabel(w.Address(), "main")
	paths := &DataPaths{WalletAddress: func() string { return w.Address() }}
	res, code, msg := execGetBalances("test", paths, &memJournal{}, nil, nil, nil, nil)
	if code != 0 {
		t.Fatalf("getbalances: %s", msg)
	}
	m := res.(map[string]interface{})
	mine := m["mine"].(map[string]interface{})
	if mine["trusted"] == nil {
		t.Fatalf("mine %#v", mine)
	}
}
