// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/store"
)

func TestFilterListUnspentMaximumCount(t *testing.T) {
	matches := []walletUtxoMatch{
		{row: store.UtxoDumpRow{Value: 1e8}, address: "a"},
		{row: store.UtxoDumpRow{Value: 3e8}, address: "b"},
		{row: store.UtxoDumpRow{Value: 2e8}, address: "c"},
	}
	out := filterListUnspentMatches(matches, listUnspentQueryOpts{maximumCount: 2})
	if len(out) != 2 {
		t.Fatalf("len %d", len(out))
	}
	if out[0].row.Value != 3e8 || out[1].row.Value != 2e8 {
		t.Fatalf("order/value %#v", out)
	}
}

func TestFilterListUnspentMinimumAmount(t *testing.T) {
	matches := []walletUtxoMatch{
		{row: store.UtxoDumpRow{Value: 5e7}, address: "a"},
		{row: store.UtxoDumpRow{Value: 2e8}, address: "b"},
	}
	out := filterListUnspentMatches(matches, listUnspentQueryOpts{minimumAmount: 1.0})
	if len(out) != 1 || out[0].address != "b" {
		t.Fatalf("%#v", out)
	}
}
