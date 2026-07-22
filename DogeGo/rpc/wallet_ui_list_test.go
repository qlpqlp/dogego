// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestWalletTxEntryMatchesKind(t *testing.T) {
	m := map[string]interface{}{"tx_kind": "mining"}
	if !walletTxEntryMatchesKind(m, "mining") {
		t.Fatal("mining")
	}
	if walletTxEntryMatchesKind(m, "sent") {
		t.Fatal("not sent")
	}
	m = map[string]interface{}{"tx_kind": "sent_pq", "pq_tag": "pq1"}
	if !walletTxEntryMatchesKind(m, "quantum") {
		t.Fatal("quantum")
	}
}

func TestWalletTxEntryMatchesSearch(t *testing.T) {
	m := map[string]interface{}{
		"txid":    "abc123def",
		"address": "DTestAddr",
		"amount":  10.5,
	}
	if !walletTxEntryMatchesSearch(m, "dtest") {
		t.Fatal("address match")
	}
	if !walletTxEntryMatchesSearch(m, "abc123") {
		t.Fatal("txid match")
	}
	if walletTxEntryMatchesSearch(m, "nomatch") {
		t.Fatal("should not match")
	}
}

func TestFilterWalletTxEntriesPaging(t *testing.T) {
	all := []interface{}{
		map[string]interface{}{"txid": "a", "tx_kind": "sent", "time": float64(100)},
		map[string]interface{}{"txid": "b", "tx_kind": "mining", "time": float64(90)},
		map[string]interface{}{"txid": "c", "tx_kind": "received", "time": float64(80)},
	}
	filtered := filterWalletTxEntries(all, "", "mining")
	if len(filtered) != 1 {
		t.Fatalf("filtered len %d", len(filtered))
	}
	page := WalletTxListPage{Total: len(filtered), Offset: 0, Limit: 1, Items: filtered[:1]}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page %#v", page)
	}
}

func TestWalletListTransactionsPageOffsetLimit(t *testing.T) {
	all := []interface{}{
		map[string]interface{}{"txid": "a", "time": float64(300)},
		map[string]interface{}{"txid": "b", "time": float64(200)},
		map[string]interface{}{"txid": "c", "time": float64(100)},
	}
	filtered := filterWalletTxEntries(all, "", "all")
	total := len(filtered)
	offset, limit := 1, 1
	end := offset + limit
	if end > total {
		end = total
	}
	page := WalletTxListPage{Total: total, Offset: offset, Limit: limit, Items: filtered[offset:end]}
	if page.Total != 3 || len(page.Items) != 1 {
		t.Fatalf("page %#v", page)
	}
	if page.Items[0].(map[string]interface{})["txid"] != "b" {
		t.Fatalf("item %#v", page.Items[0])
	}
}
