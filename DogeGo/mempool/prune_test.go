// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"testing"
	"time"

	"dogego/wire"
)

func TestPruneExpired(t *testing.T) {
	p := New(100)
	p.SetPolicy(300, 1)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Script: []byte{0x01}}},
		Vout:    []wire.TxOut{{Value: 1_000_000, PkScript: []byte{0x51}}},
	}
	raw := tx.SerializeForHash()
	if err := p.Add(raw); err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	for id := range p.addedAt {
		p.addedAt[id] = time.Now().Unix() - 7200
	}
	p.mu.Unlock()
	if n := p.PruneExpired(); n != 1 {
		t.Fatalf("pruned %d want 1", n)
	}
	if p.Count() != 0 {
		t.Fatalf("count %d", p.Count())
	}
}

func TestMempoolByteCapIsFull(t *testing.T) {
	p := New(100)
	p.SetPolicy(1, 24)
	p.mu.Lock()
	p.raw["x"] = make([]byte, 1_100_000)
	p.mu.Unlock()
	if !p.IsFull() {
		t.Fatal("expected byte cap to mark pool full")
	}
}
