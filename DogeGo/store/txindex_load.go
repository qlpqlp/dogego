// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"fmt"

	"dogego/wire"
)

// RawBlockGetter loads serialized block payloads by block hash (LE).
type RawBlockGetter interface {
	Get(hashLE [32]byte) ([]byte, error)
}

// LoadIndexedTx returns a confirmed tx by RPC txid using the tx index (v2 embedded raw when present).
func LoadIndexedTx(ix *TxIndex, raw RawBlockGetter, txidHex string) (*wire.Tx, error) {
	if ix == nil {
		return nil, fmt.Errorf("tx index disabled")
	}
	hit, err := ix.LookupHit(txidHex)
	if err != nil {
		return nil, err
	}
	if len(hit.TxRaw) > 0 {
		return wire.DeserializeTx(hit.TxRaw)
	}
	if raw == nil {
		return nil, fmt.Errorf("raw block store required for legacy tx index entry")
	}
	payload, err := raw.Get(hit.BlockHashLE)
	if err != nil {
		return nil, err
	}
	tx, _, err := wire.ReadTxAtIndex(payload, hit.TxIndex)
	return tx, err
}

// LoadIndexedTxVout returns value and scriptPubKey for a confirmed outpoint via the tx index.
func LoadIndexedTxVout(ix *TxIndex, raw RawBlockGetter, txidHex string, vout uint32) (value int64, pkScript []byte, ok bool) {
	tx, err := LoadIndexedTx(ix, raw, txidHex)
	if err != nil || int(vout) >= len(tx.Vout) {
		return 0, nil, false
	}
	o := tx.Vout[vout]
	return o.Value, append([]byte(nil), o.PkScript...), true
}
