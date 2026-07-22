// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import "testing"

func TestSortAddressEntriesPathTypeOrder(t *testing.T) {
	rows := []AddressEntry{
		{Address: "DChange1", HDPath: "m/44'/3'/0'/1/1", IsChange: true},
		{Address: "DWatch", WatchOnly: true},
		{Address: "DRecv2", HDPath: "m/44'/3'/0'/0/2"},
		{Address: "DRecv0", HDPath: "m/44'/3'/0'/0/0"},
		{Address: "DNodeTip", HDPath: "m/44'/3'/0'/2/0", IsNodeTip: true},
		{Address: "DCosigner", IsCosigner: true},
		{Address: "DChange0", HDPath: "m/44'/3'/0'/1/0", IsChange: true},
		{Address: "DImport", Label: "imported"},
	}
	SortAddressEntries(rows)
	want := []string{"DRecv0", "DRecv2", "DNodeTip", "DChange0", "DChange1", "DWatch", "DCosigner", "DImport"}
	for i, addr := range want {
		if rows[i].Address != addr {
			t.Fatalf("index %d: got %s want %s (full %#v)", i, rows[i].Address, addr, rows)
		}
	}
}
