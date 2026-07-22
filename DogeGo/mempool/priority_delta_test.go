// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"testing"

	"dogego/wire"
)

func TestPoolFeeDelta(t *testing.T) {
	p := New(100)
	raw := minimalCoinbaseRaw(t)
	if err := p.Add(raw); err != nil {
		t.Fatal(err)
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	id := TxIDDisplayHex(tx.TxHash())
	if err := p.AddFeeDelta(id, 5000); err != nil {
		t.Fatal(err)
	}
	fees := map[string]int64{id: 100}
	p.ApplyFeeDeltas(fees)
	if fees[id] != 5100 {
		t.Fatalf("fee %d", fees[id])
	}
	if got := p.FeeDeltaKoinu(id); got != 5000 {
		t.Fatalf("FeeDeltaKoinu=%d", got)
	}
}

func TestPrioritiseFeeDeltaPropagatesToAncestor(t *testing.T) {
	p := New(10)
	parentRaw := minimalCoinbaseRaw(t)
	parent, _ := wire.DeserializeTx(parentRaw)
	child := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: parent.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x52}}},
	}
	childRaw, _ := child.Serialize()
	_ = p.Add(parentRaw)
	_ = p.Add(childRaw)
	parentID := TxIDDisplayHex(parent.TxHash())
	childID := TxIDDisplayHex(child.TxHash())
	if err := p.AddFeeDelta(childID, 1_000_000); err != nil {
		t.Fatal(err)
	}
	st, err := p.PackageStatsForTxID(parentID, map[string]int64{parentID: 100, childID: 1}, map[string]int{parentID: 100, childID: 100})
	if err != nil {
		t.Fatal(err)
	}
	if got := p.MiningAncestorFeesKoinu(st, parentID); got < st.AncestorFeesKoinu+1_000_000 {
		t.Fatalf("parent mining fees %d", got)
	}
}

func TestClearFeeDeltaOnRemove(t *testing.T) {
	p := New(10)
	raw := minimalCoinbaseRaw(t)
	if err := p.Add(raw); err != nil {
		t.Fatal(err)
	}
	tx, _ := wire.DeserializeTx(raw)
	id := TxIDDisplayHex(tx.TxHash())
	_ = p.AddFeeDelta(id, 1000)
	if !p.RemoveByTxID(id) {
		t.Fatal("remove")
	}
	if p.FeeDeltaKoinu(id) != 0 {
		t.Fatalf("delta after remove %d", p.FeeDeltaKoinu(id))
	}
}

func TestLatentFeeDeltaBeforeMempoolAdmit(t *testing.T) {
	p := New(10)
	raw := minimalCoinbaseRaw(t)
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	id := TxIDDisplayHex(tx.TxHash())
	if err := p.AddFeeDelta(id, 7000); err != nil {
		t.Fatal(err)
	}
	if p.FeeDeltaKoinu(id) != 7000 {
		t.Fatalf("latent delta %d", p.FeeDeltaKoinu(id))
	}
	if err := p.Add(raw); err != nil {
		t.Fatal(err)
	}
	fees := map[string]int64{id: 100}
	p.ApplyFeeDeltas(fees)
	if fees[id] != 7100 {
		t.Fatalf("applied fee %d", fees[id])
	}
}

func TestFeeDeltaZeroRemovesEntry(t *testing.T) {
	p := New(10)
	raw := minimalCoinbaseRaw(t)
	_ = p.Add(raw)
	tx, _ := wire.DeserializeTx(raw)
	id := TxIDDisplayHex(tx.TxHash())
	_ = p.AddFeeDelta(id, 1000)
	if err := p.AddFeeDelta(id, -1000); err != nil {
		t.Fatal(err)
	}
	if p.FeeDeltaKoinu(id) != 0 {
		t.Fatalf("delta %d", p.FeeDeltaKoinu(id))
	}
	if len(p.ExportFeeDeltas()) != 0 {
		t.Fatalf("export %#v", p.ExportFeeDeltas())
	}
}
