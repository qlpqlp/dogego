// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"
)

func TestBuildCoinbaseTxBIP34Height(t *testing.T) {
	tx := BuildCoinbaseTx(708658, 500_000*KoinuPerCoin, P2PKHPkScript([20]byte{1}))
	h, ok := CoinbaseHeightFromScript(tx.Vin[0].Script)
	if !ok || h != 708658 {
		t.Fatalf("height %d ok=%v", h, ok)
	}
}
