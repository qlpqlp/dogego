// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// UtxoOutpointSource resolves confirmed unspent outputs at the chain tip (in-memory UTXO cache).
type UtxoOutpointSource interface {
	UnspentOutpoint(prevHash [32]byte, vout uint32) (value int64, pkScript []byte, ok bool)
}

// UtxoPrevOutView adapts UtxoOutpointSource to PrevOutView.
type UtxoPrevOutView struct {
	Source UtxoOutpointSource
}

// Lookup implements PrevOutView.
func (v UtxoPrevOutView) Lookup(prevHash [32]byte, idx uint32) (PrevOut, bool) {
	if v.Source == nil {
		return PrevOut{}, false
	}
	val, pk, ok := v.Source.UnspentOutpoint(prevHash, idx)
	if !ok {
		return PrevOut{}, false
	}
	return PrevOut{Value: val, PkScript: pk}, true
}

// UnspentHeight implements utxoHeightLookup when Source tracks confirmation height.
func (v UtxoPrevOutView) UnspentHeight(prevHash [32]byte, idx uint32) (int64, bool) {
	if v.Source == nil {
		return 0, false
	}
	if hs, ok := v.Source.(interface {
		UnspentHeight([32]byte, uint32) (int64, bool)
	}); ok {
		return hs.UnspentHeight(prevHash, idx)
	}
	return 0, false
}
