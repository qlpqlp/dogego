// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/mempool"
	"dogego/wire"
)

// DifferentialMaturityJournal implements header journal surfaces for coinbase maturity vectors.
type DifferentialMaturityJournal struct {
	CoinHeight int64
	Tip        int64
}

func (m *DifferentialMaturityJournal) TipHeight() (int64, error) { return m.Tip, nil }
func (m *DifferentialMaturityJournal) ReadHeaderAt(int64) ([]byte, error) {
	return make([]byte, 80), nil
}
func (m *DifferentialMaturityJournal) HeightByDisplayHash(string) (int64, error) {
	return m.CoinHeight, nil
}

// DifferentialMaturityTxIndex resolves the immature coinbase txid for maturity vectors.
type DifferentialMaturityTxIndex struct {
	CoinbaseID [32]byte
}

func (m DifferentialMaturityTxIndex) Lookup(txidHex string) ([32]byte, uint32, error) {
	if txidDisplayFromLE(m.CoinbaseID) == txidHex {
		return [32]byte{0xaa}, 0, nil
	}
	return [32]byte{}, 0, fmt.Errorf("missing")
}

// DifferentialCLTVJournal provides a high tip for BIP65-active CLTV mempool vectors.
type DifferentialCLTVJournal struct {
	Tip int64
}

func (m *DifferentialCLTVJournal) TipHeight() (int64, error) { return m.Tip, nil }
func (m *DifferentialCLTVJournal) ReadHeaderAt(int64) ([]byte, error) {
	return make([]byte, 80), nil
}
func (m *DifferentialCLTVJournal) HeightByDisplayHash(string) (int64, error) {
	return m.Tip, nil
}

// CoinbaseImmatureDifferentialSpend returns the spend tx and stubs for core_mempool_vectors coinbase_immature.
func CoinbaseImmatureDifferentialSpend() (*wire.Tx, DifferentialMaturityTxIndex, *DifferentialMaturityJournal, mempoolStubPrevOutView) {
	var coinID [32]byte
	coinID[0] = 0xcb
	pad := make([]byte, 24)
	pk := fixtureP2PKHScript()
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: coinID, PrevIdx: 0, Sequence: 0xffffffff, Script: pad}},
		Vout:    []wire.TxOut{{Value: HardDustLimitKoinu + 1_000_000, PkScript: pk}},
	}
	const coinH = int64(200_000)
	ix := DifferentialMaturityTxIndex{CoinbaseID: coinID}
	j := &DifferentialMaturityJournal{CoinHeight: coinH, Tip: coinH + 100}
	view := mempoolStubPrevOutView{}
	view[outpointKey(coinID, 0)] = PrevOut{Value: 50_000_000, PkScript: pk}
	return spend, ix, j, view
}

// NonFinalDifferentialSpend returns a height-locked tx and journal stub for non-final mempool vectors.
func NonFinalDifferentialSpend() (*wire.Tx, *DifferentialMaturityJournal, mempoolStubPrevOutView) {
	pad := make([]byte, 24)
	pk := fixtureP2PKHScript()
	spend := &wire.Tx{
		Version:  1,
		LockTime: 52,
		Vin:      []wire.TxIn{{PrevHash: [32]byte{0xf4}, PrevIdx: 0, Sequence: wire.SequenceFinal - 1, Script: pad}},
		Vout:     []wire.TxOut{{Value: HardDustLimitKoinu + 1_000_000, PkScript: pk}},
	}
	j := &DifferentialMaturityJournal{Tip: 50}
	view := mempoolStubPrevOutView{}
	view[outpointKey([32]byte{0xf4}, 0)] = PrevOut{Value: 50_000_000, PkScript: pk}
	return spend, j, view
}

// BuildRBFNotReplaceableDifferentialFixture builds a conflict with a non-BIP125-signaling mempool parent (Core opt-in RBF reject).
func BuildRBFNotReplaceableDifferentialFixture() (raw []byte, prep func(*mempool.Pool) error, view fixedPrevOutView, err error) {
	const parentVal = int64(200_000_000)
	parentHash := [32]byte{9}
	old := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: fixtureP2PKHScript()}},
	}
	pk := fixtureP2PKHScript()
	newTx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: pk}},
	}
	parentID := txidDisplayFromLE(parentHash)
	view = fixedPrevOutView{
		rpcOutpointKey(parentID, 0): {Value: parentVal, PkScript: pk},
	}
	oldRaw, err := old.Serialize()
	if err != nil {
		return nil, nil, nil, err
	}
	raw, err = newTx.Serialize()
	if err != nil {
		return nil, nil, nil, err
	}
	prep = func(pool *mempool.Pool) error {
		return pool.Add(oldRaw)
	}
	return raw, prep, view, nil
}

// BuildRBFFullRBFAcceptFixture reuses the non-signaling conflict; admitted when FullRBF is enabled (Core -mempoolfullrbf).
func BuildRBFFullRBFAcceptFixture() (raw []byte, prep func(*mempool.Pool) error, view fixedPrevOutView, err error) {
	return BuildRBFNotReplaceableDifferentialFixture()
}

// BuildRBFSufficientFeeDifferentialFixture builds a BIP125 replacement that pays a higher package fee (Core accept path).
func BuildRBFSufficientFeeDifferentialFixture() (raw []byte, prep func(*mempool.Pool) error, view fixedPrevOutView, err error) {
	const parentVal = int64(200_000_000)
	parentHash := [32]byte{9}
	old := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: fixtureP2PKHScript()}},
	}
	pk := fixtureP2PKHScript()
	newTx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: pk}}, // 150M fee vs old 100M
	}
	parentID := txidDisplayFromLE(parentHash)
	view = fixedPrevOutView{
		rpcOutpointKey(parentID, 0): {Value: parentVal, PkScript: pk},
	}
	oldRaw, err := old.Serialize()
	if err != nil {
		return nil, nil, nil, err
	}
	raw, err = newTx.Serialize()
	if err != nil {
		return nil, nil, nil, err
	}
	prep = func(pool *mempool.Pool) error {
		return pool.Add(oldRaw)
	}
	return raw, prep, view, nil
}

// BuildRBFInsufficientFeeDifferentialFixture builds the replacement tx and pool prep for the RBF vector.
func BuildRBFInsufficientFeeDifferentialFixture() (raw []byte, prep func(*mempool.Pool) error, view fixedPrevOutView, err error) {
	const parentVal = int64(200_000_000)
	parentHash := [32]byte{9}
	old := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: fixtureP2PKHScript()}},
	}
	pad := make([]byte, 24)
	pk := fixtureP2PKHScript()
	newTx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: parentHash,
			PrevIdx:  0,
			Sequence: wire.MaxBIP125RBFSequence,
			Script:   pad,
		}},
		Vout: []wire.TxOut{{Value: 199_950_000, PkScript: pk}},
	}
	parentID := txidDisplayFromLE(parentHash)
	view = fixedPrevOutView{
		rpcOutpointKey(parentID, 0): {Value: parentVal, PkScript: pk},
	}
	oldRaw, err := old.Serialize()
	if err != nil {
		return nil, nil, nil, err
	}
	raw, err = newTx.Serialize()
	if err != nil {
		return nil, nil, nil, err
	}
	prep = func(pool *mempool.Pool) error {
		return pool.Add(oldRaw)
	}
	return raw, prep, view, nil
}

// BuildRBFTTooManyDescendantsFixture builds a BIP125 replacement blocked by descendant limit on the conflict cluster.
func BuildRBFTTooManyDescendantsFixture() (raw []byte, prep func(*mempool.Pool) error, view fixedPrevOutView, err error) {
	const parentVal = int64(500_000_000)
	var root [32]byte
	root[0] = 0xa1
	pk := fixtureP2PKHScript()
	view = fixedPrevOutView{
		rpcOutpointKey(txidDisplayFromLE(root), 0): {Value: parentVal, PkScript: pk},
	}
	old := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: root, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 400_000_000, PkScript: pk}},
	}
	newTx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: root, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: pk}},
	}
	oldRaw, err := old.Serialize()
	if err != nil {
		return nil, nil, nil, err
	}
	raw, err = newTx.Serialize()
	if err != nil {
		return nil, nil, nil, err
	}
	prep = func(pool *mempool.Pool) error {
		if err := pool.Add(oldRaw); err != nil {
			return err
		}
		prevHash := old.TxHash()
		for i := 0; i < 26; i++ {
			child := &wire.Tx{
				Version: 1,
				Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
				Vout:    []wire.TxOut{{Value: 40_000_000, PkScript: pk}},
			}
			childRaw, err := child.Serialize()
			if err != nil {
				return err
			}
			if err := pool.Add(childRaw); err != nil {
				return err
			}
			prevHash = child.TxHash()
		}
		return nil
	}
	return raw, prep, view, nil
}

const rbfRule5ChainDescendants = 24
const rbfRule5ConflictCount = 5

// BuildRBFTooManyConflictsFixture builds a replacement that would evict more than 100 mempool
// transactions (BIP125 rule 5): five direct conflicts each with 24 in-mempool descendants.
func BuildRBFTooManyConflictsFixture() (raw []byte, prep func(*mempool.Pool) error, view fixedPrevOutView, err error) {
	pk := fixtureP2PKHScript()
	const parentVal = int64(500_000_000)
	view = fixedPrevOutView{}
	var newVins []wire.TxIn
	var oldRaws [][]byte
	for i := 0; i < rbfRule5ConflictCount; i++ {
		var root [32]byte
		root[0] = byte(0x40 + i)
		rootID := txidDisplayFromLE(root)
		view[rpcOutpointKey(rootID, 0)] = PrevOut{Value: parentVal, PkScript: pk}
		old := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: root, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
			Vout:    []wire.TxOut{{Value: 400_000_000, PkScript: pk}},
		}
		oldRaw, err := old.Serialize()
		if err != nil {
			return nil, nil, nil, err
		}
		oldRaws = append(oldRaws, oldRaw)
		newVins = append(newVins, wire.TxIn{
			PrevHash: root, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence,
		})
	}
	newTx := &wire.Tx{
		Version: 1,
		Vin:     newVins,
		Vout:    []wire.TxOut{{Value: 10_000_000, PkScript: pk}},
	}
	raw, err = newTx.Serialize()
	if err != nil {
		return nil, nil, nil, err
	}
	prep = func(pool *mempool.Pool) error {
		for _, oldRaw := range oldRaws {
			if err := pool.Add(oldRaw); err != nil {
				return err
			}
			tx, err := wire.DeserializeTx(oldRaw)
			if err != nil {
				return err
			}
			prevHash := tx.TxHash()
			for c := 0; c < rbfRule5ChainDescendants; c++ {
				child := &wire.Tx{
					Version: 1,
					Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
					Vout:    []wire.TxOut{{Value: 30_000_000, PkScript: pk}},
				}
				childRaw, err := child.Serialize()
				if err != nil {
					return err
				}
				if err := pool.Add(childRaw); err != nil {
					return err
				}
				prevHash = child.TxHash()
			}
		}
		return nil
	}
	return raw, prep, view, nil
}

// BuildRBFNewUnconfirmedInputFixture builds a BIP125 replacement that adds a fresh unconfirmed
// parent input not spent by the directly conflicting transaction (BIP125 rule 2 reject).
func BuildRBFNewUnconfirmedInputFixture() (raw []byte, prep func(*mempool.Pool) error, view fixedPrevOutView, err error) {
	pk := fixtureP2PKHScript()
	confHash := [32]byte{9}
	confID := txidDisplayFromLE(confHash)
	var unconfFunding [32]byte
	unconfFunding[0] = 0xaa
	fundingID := txidDisplayFromLE(unconfFunding)
	unconfParent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: unconfFunding, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 200_000_000, PkScript: pk}},
	}
	unconfRaw, err := unconfParent.Serialize()
	if err != nil {
		return nil, nil, nil, err
	}
	unconfID := txidDisplayFromLE(unconfParent.TxHash())
	old := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: confHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
		Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: pk}},
	}
	oldRaw, err := old.Serialize()
	if err != nil {
		return nil, nil, nil, err
	}
	newTx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{
			{PrevHash: confHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence},
			{PrevHash: unconfParent.TxHash(), PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence},
		},
		Vout: []wire.TxOut{{Value: 50_000_000, PkScript: pk}},
	}
	raw, err = newTx.Serialize()
	if err != nil {
		return nil, nil, nil, err
	}
	view = fixedPrevOutView{
		rpcOutpointKey(confID, 0):    {Value: 200_000_000, PkScript: pk},
		rpcOutpointKey(unconfID, 0):  {Value: 200_000_000, PkScript: pk},
		rpcOutpointKey(fundingID, 0): {Value: 500_000_000, PkScript: pk},
	}
	prep = func(pool *mempool.Pool) error {
		if err := pool.Add(unconfRaw); err != nil {
			return err
		}
		return pool.Add(oldRaw)
	}
	return raw, prep, view, nil
}

// NonBIP68FinalDifferentialSpend returns a CSV-relative-lock spend rejected at the next block.
func NonBIP68FinalDifferentialSpend() (*wire.Tx, *DifferentialCLTVJournal, differentialHeightPrevOutView) {
	var prev [32]byte
	prev[0] = 0xe1
	pad := make([]byte, 24)
	pk := fixtureP2PKHScript()
	spend := &wire.Tx{
		Version:  2,
		LockTime: 0,
		Vin:      []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 10, Script: pad}},
		Vout:     []wire.TxOut{{Value: HardDustLimitKoinu + 1_000_000, PkScript: pk}},
	}
	j := &DifferentialCLTVJournal{Tip: 500_000}
	view := differentialHeightPrevOutView{
		mempoolStubPrevOutView: mempoolStubPrevOutView{
			outpointKey(prev, 0): {Value: 50_000_000, PkScript: pk},
		},
		heights: map[[36]byte]int64{outpointKey(prev, 0): 499_995},
	}
	return spend, j, view
}

type differentialHeightPrevOutView struct {
	mempoolStubPrevOutView
	heights map[[36]byte]int64
}

func (v differentialHeightPrevOutView) UnspentHeight(prevHash [32]byte, vout uint32) (int64, bool) {
	h, ok := v.heights[outpointKey(prevHash, vout)]
	return h, ok
}
