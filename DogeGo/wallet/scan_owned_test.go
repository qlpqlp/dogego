// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import "testing"

func TestSeedOwnedFromPriorReceives(t *testing.T) {
	scriptA := []byte{0x76, 0xa9, 0x14, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 0x88, 0xac}
	scriptSet := map[string][]byte{string(scriptA): scriptA}
	addrByScript := map[string]string{string(scriptA): "DAddrA"}
	owned := make(map[[36]byte]walletCoin)
	prior := []ScannedTx{{
		TxID:        "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
		Category:    "receive",
		Address:     "DAddrA",
		AmountKoinu: 10_000_000_000,
		Vout:        0,
		BlockHeight: 10,
	}}
	seedOwnedFromPriorReceives(owned, prior, scriptSet, addrByScript)
	if len(owned) != 1 {
		t.Fatalf("owned=%d want 1", len(owned))
	}
}
