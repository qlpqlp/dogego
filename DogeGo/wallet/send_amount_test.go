// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"testing"

	"dogego/wire"
)

func TestSendDisplayKoinuSelfTransfer(t *testing.T) {
	scriptA := []byte("wallet-a")
	scriptB := []byte("wallet-b")
	tracked := map[string][]byte{
		string(scriptA): scriptA,
		string(scriptB): scriptB,
	}
	vouts := []wire.TxOut{
		{Value: 5_500_000_000, PkScript: scriptB},
		{Value: 4_499_000_000, PkScript: scriptA},
	}
	got := SendDisplayKoinu(10_000_000_000, map[string]int64{
		"addr-b": 5_500_000_000,
		"addr-a": 4_499_000_000,
	}, "addr-a", vouts, tracked)
	if got != 5_500_000_000 {
		t.Fatalf("self-transfer got %d want 5500000000", got)
	}
}

func TestSendDisplayKoinuExternal(t *testing.T) {
	scriptA := []byte("wallet-a")
	tracked := map[string][]byte{string(scriptA): scriptA}
	vouts := []wire.TxOut{
		{Value: 5_500_000_000, PkScript: []byte("external")},
		{Value: 4_499_000_000, PkScript: scriptA},
	}
	got := SendDisplayKoinu(10_000_000_000, map[string]int64{"addr-a": 4_499_000_000}, "addr-a", vouts, tracked)
	if got != 5_500_000_000 {
		t.Fatalf("external send got %d want 5500000000", got)
	}
}

func TestSendDisplayKoinuChangeToDifferentOwnedAddress(t *testing.T) {
	scriptA := []byte("wallet-a")
	scriptB := []byte("wallet-b")
	external := []byte("external-payee")
	tracked := map[string][]byte{
		string(scriptA): scriptA,
		string(scriptB): scriptB,
	}
	vouts := []wire.TxOut{
		{Value: 4_499_000_000, PkScript: scriptB},
		{Value: 5_500_000_000, PkScript: external},
	}
	got := SendDisplayKoinu(10_000_000_000, map[string]int64{
		"addr-b": 4_499_000_000,
	}, "addr-a", vouts, tracked)
	if got != 5_500_000_000 {
		t.Fatalf("change to other owned addr got %d want 5500000000 (payment not change)", got)
	}
}

func TestSendDisplayKoinuConsolidationFeeOnly(t *testing.T) {
	scriptA := []byte("wallet-a")
	tracked := map[string][]byte{string(scriptA): scriptA}
	vouts := []wire.TxOut{{Value: 9_999_000_000, PkScript: scriptA}}
	got := SendDisplayKoinu(10_000_000_000, map[string]int64{"addr-a": 9_999_000_000}, "addr-a", vouts, tracked)
	if got != 1_000_000 {
		t.Fatalf("consolidation got %d want 1000000 fee", got)
	}
}
