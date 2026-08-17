// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"fmt"

	"dogego/wire"
)

// PrevOut is the output being spent (value + scriptPubKey).
type PrevOut struct {
	Value    int64
	PkScript []byte
	Height   int64 // confirmation height; 0 when unknown (mempool / chain scan without UTXO)
	Coinbase bool  // Core CCoins::fCoinBase; false when unknown
}

// PrevOutView resolves prevouts for script verification.
type PrevOutView interface {
	Lookup(prevHash [32]byte, idx uint32) (PrevOut, bool)
}

// MempoolPrevOutView resolves outputs from transactions already in the mempool.
type MempoolPrevOutView struct {
	Pool MempoolPool
}

// Lookup implements PrevOutView.
func (v *MempoolPrevOutView) Lookup(prevHash [32]byte, idx uint32) (PrevOut, bool) {
	if v == nil || isNilIface(v.Pool) {
		return PrevOut{}, false
	}
	txid := txidDisplayFromLE(prevHash)
	raw, err := v.Pool.GetRawByTxID(txid)
	if err != nil {
		return PrevOut{}, false
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil || int(idx) >= len(tx.Vout) {
		return PrevOut{}, false
	}
	o := tx.Vout[idx]
	return PrevOut{Value: o.Value, PkScript: append([]byte(nil), o.PkScript...)}, true
}

func txidDisplayFromLE(h [32]byte) string {
	b := make([]byte, 32)
	for i := 0; i < 32; i++ {
		b[i] = h[31-i]
	}
	return hex.EncodeToString(b)
}

// DecodeDisplayTxid parses an RPC display txid (big-endian hex) into wire prevout hash (LE).
func DecodeDisplayTxid(display string, out *[32]byte) error {
	if out == nil {
		return fmt.Errorf("decode txid: nil out")
	}
	b, err := hex.DecodeString(display)
	if err != nil || len(b) != 32 {
		return fmt.Errorf("decode txid: want 64 hex chars")
	}
	for i := 0; i < 32; i++ {
		out[i] = b[31-i]
	}
	return nil
}
