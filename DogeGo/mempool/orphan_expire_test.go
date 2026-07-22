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

func TestOrphanPruneExpired(t *testing.T) {
	o := NewOrphanPool(10)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Script: []byte{0x01}}},
		Vout:    []wire.TxOut{{Value: 0, PkScript: []byte{0x51}}},
	}
	raw := tx.SerializeForHash()
	if _, err := o.Add(raw, []string{"deadbeef"}, ""); err != nil {
		t.Fatal(err)
	}
	o.mu.Lock()
	for id := range o.expiresAt {
		o.expiresAt[id] = time.Now().Unix() - 1
	}
	o.mu.Unlock()
	if n := o.PruneExpired(); n != 1 {
		t.Fatalf("pruned %d", n)
	}
	if o.Count() != 0 {
		t.Fatal("expected empty")
	}
}

func TestOrphanRejectsHighWeight(t *testing.T) {
	o := NewOrphanPool(10)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Script: make([]byte, 100_001)}},
		Vout:    []wire.TxOut{{Value: 0, PkScript: []byte{0x51}}},
	}
	raw := tx.SerializeForHash()
	if _, err := o.Add(raw, []string{"a"}, ""); err == nil {
		t.Fatal("expected weight error")
	}
}
