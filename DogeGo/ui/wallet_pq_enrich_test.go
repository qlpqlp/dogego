// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/hex"
	"testing"

	"dogego/consensus"
	"dogego/wire"
)

func TestWalletSendPQMetaFromHex(t *testing.T) {
	script := make([]byte, 38)
	script[0] = 0x6a
	script[1] = 0x24
	copy(script[2:6], []byte(consensus.PQTagFalcon))
	for i := 6; i < 38; i++ {
		script[i] = byte(i)
	}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout: []wire.TxOut{
			{Value: 1, PkScript: []byte{0x51}},
			{Value: 0, PkScript: script},
		},
	}
	raw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	kind, tag := walletSendPQMetaFromHex(hex.EncodeToString(raw))
	if kind != "sent_pq" || tag != consensus.PQTagFalcon {
		t.Fatalf("got %q %q", kind, tag)
	}
}

func TestWalletHistoryEntryMatchesKindQuantum(t *testing.T) {
	pq := map[string]interface{}{"tx_kind": "sent_pq", "pq_tag": "FLC1"}
	if !walletHistoryEntryMatchesKind(pq, "quantum") {
		t.Fatal("sent_pq should match quantum")
	}
	recv := map[string]interface{}{"tx_kind": "received_pq", "pq_tag": "FLC1"}
	if !walletHistoryEntryMatchesKind(recv, "quantum") {
		t.Fatal("received_pq should match quantum")
	}
	plain := map[string]interface{}{"tx_kind": "sent"}
	if walletHistoryEntryMatchesKind(plain, "quantum") {
		t.Fatal("plain sent should not match quantum")
	}
}

func TestWalletReceivePQMetaFromHex(t *testing.T) {
	script := make([]byte, 38)
	script[0] = 0x6a
	script[1] = 0x24
	copy(script[2:6], []byte(consensus.PQTagFalcon))
	for i := 6; i < 38; i++ {
		script[i] = byte(i)
	}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout: []wire.TxOut{
			{Value: 5_000_000_000, PkScript: []byte{0x76, 0xa9, 0x14}},
			{Value: 0, PkScript: script},
		},
	}
	raw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	kind, tag := walletPQMetaFromHex(hex.EncodeToString(raw), "receive")
	if kind != "received_pq" || tag != consensus.PQTagFalcon {
		t.Fatalf("got %q %q", kind, tag)
	}
}
