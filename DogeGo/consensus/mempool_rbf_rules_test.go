// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"testing"

	"dogego/wire"
)

// rbfGraphMockPool adds descendant counts to rbfMockPool for BIP125 rule-5 tests.
type rbfGraphMockPool struct {
	*rbfMockPool
	descendants map[string]int
}

func (m *rbfGraphMockPool) MempoolDescendantCount(id string) (int, error) {
	return m.descendants[id], nil
}

// TestRBFRule5TooManyConflicts rejects a replacement that would evict more than 100 transactions.
func TestRBFRule5TooManyConflicts(t *testing.T) {
	const parentVal = int64(200_000_000)
	parentHash := [32]byte{9}
	parentID := txidDisplayFromLE(parentHash)
	view := fixedPrevOutView{
		rpcOutpointKey(parentID, 0): {Value: parentVal, PkScript: []byte{0x51}},
	}
	old := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: []byte{0x51}}},
	}
	newTx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: []byte{0x51}}},
	}
	oldRaw, _ := old.Serialize()
	oldID := txidDisplayFromLE(old.TxHash())
	base := &rbfMockPool{
		spend: map[string]string{rpcOutpointKey(parentID, 0): oldID},
		raw:   map[string][]byte{oldID: oldRaw},
	}
	pool := &rbfGraphMockPool{
		rbfMockPool: base,
		descendants: map[string]int{oldID: MaxRBFReplacementCandidates + 5},
	}
	err := TryResolveMempoolRBFConflicts(newTx, pool, view, false)
	// The descendant-count guard trips first when the conflict itself exceeds the descendant
	// limit; ensure we reject via one of the BIP125 package guards, not accept.
	if err == nil {
		t.Fatal("expected rejection for oversized conflict cluster")
	}
	if !errors.Is(err, ErrRBFTooManyConflicts) && !errors.Is(err, ErrRBFTxTooManyDescendants) {
		t.Fatalf("expected rule-5/descendant rejection, got %v", err)
	}
}

// TestRBFRule5ManySmallConflictsExceeds sums conflicts + descendants across several conflicts.
func TestRBFRule5ManySmallConflictsExceeds(t *testing.T) {
	pk := []byte{0x51}
	view := fixedPrevOutView{}
	spend := map[string]string{}
	raw := map[string][]byte{}
	descendants := map[string]int{}
	var newVins []wire.TxIn
	// 5 distinct conflicts, each carrying 24 descendants -> 5*(1+24)=125 > 100.
	for i := 0; i < 5; i++ {
		var ph [32]byte
		ph[0] = byte(0x30 + i)
		pid := txidDisplayFromLE(ph)
		view[rpcOutpointKey(pid, 0)] = PrevOut{Value: 200_000_000, PkScript: pk}
		old := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: ph, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
			Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: pk}},
		}
		oldRaw, _ := old.Serialize()
		oldID := txidDisplayFromLE(old.TxHash())
		spend[rpcOutpointKey(pid, 0)] = oldID
		raw[oldID] = oldRaw
		descendants[oldID] = 24
		newVins = append(newVins, wire.TxIn{PrevHash: ph, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence})
	}
	newTx := &wire.Tx{Version: 1, Vin: newVins, Vout: []wire.TxOut{{Value: 10_000_000, PkScript: pk}}}
	pool := &rbfGraphMockPool{
		rbfMockPool: &rbfMockPool{spend: spend, raw: raw},
		descendants: descendants,
	}
	err := TryResolveMempoolRBFConflicts(newTx, pool, view, false)
	if !errors.Is(err, ErrRBFTooManyConflicts) {
		t.Fatalf("expected ErrRBFTooManyConflicts, got %v", err)
	}
}

// TestRBFRule2NewUnconfirmedInput rejects a replacement that adds a fresh unconfirmed parent.
func TestRBFRule2NewUnconfirmedInput(t *testing.T) {
	pk := []byte{0x51}
	// Confirmed parent spent by both old and new.
	confHash := [32]byte{9}
	confID := txidDisplayFromLE(confHash)
	// A brand-new unconfirmed parent that only the replacement spends.
	unconfHash := [32]byte{7}
	unconfID := txidDisplayFromLE(unconfHash)

	view := fixedPrevOutView{
		rpcOutpointKey(confID, 0):   {Value: 200_000_000, PkScript: pk},
		rpcOutpointKey(unconfID, 0): {Value: 200_000_000, PkScript: pk},
	}

	// The unconfirmed parent lives in the mempool.
	unconfParent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 200_000_000, PkScript: pk}},
	}
	unconfRaw, _ := unconfParent.Serialize()

	old := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: confHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: pk}},
	}
	oldRaw, _ := old.Serialize()
	oldID := txidDisplayFromLE(old.TxHash())

	newTx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{
			{PrevHash: confHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence},
			{PrevHash: unconfHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence},
		},
		Vout: []wire.TxOut{{Value: 50_000_000, PkScript: pk}},
	}
	pool := &rbfMockPool{
		spend: map[string]string{rpcOutpointKey(confID, 0): oldID},
		raw:   map[string][]byte{oldID: oldRaw, unconfID: unconfRaw},
	}
	err := TryResolveMempoolRBFConflicts(newTx, pool, view, false)
	if !errors.Is(err, ErrRBFNewUnconfirmedInput) {
		t.Fatalf("expected ErrRBFNewUnconfirmedInput, got %v", err)
	}
}

// TestRBFRule2AllowsSharedUnconfirmedInput permits an unconfirmed input the conflict already spends.
func TestRBFRule2AllowsSharedUnconfirmedInput(t *testing.T) {
	pk := []byte{0x51}
	unconfHash := [32]byte{7}
	unconfID := txidDisplayFromLE(unconfHash)
	view := fixedPrevOutView{
		rpcOutpointKey(unconfID, 0): {Value: 200_000_000, PkScript: pk},
	}
	unconfParent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 200_000_000, PkScript: pk}},
	}
	unconfRaw, _ := unconfParent.Serialize()
	old := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: unconfHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: pk}},
	}
	oldRaw, _ := old.Serialize()
	oldID := txidDisplayFromLE(old.TxHash())
	newTx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: unconfHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 20_000_000, PkScript: pk}}, // higher fee
	}
	pool := &rbfMockPool{
		spend: map[string]string{rpcOutpointKey(unconfID, 0): oldID},
		raw:   map[string][]byte{oldID: oldRaw, unconfID: unconfRaw},
	}
	if err := TryResolveMempoolRBFConflicts(newTx, pool, view, false); err != nil {
		t.Fatalf("shared unconfirmed input should be allowed, got %v", err)
	}
}
