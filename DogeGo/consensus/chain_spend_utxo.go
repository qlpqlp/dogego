// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/store"
	"dogego/wire"
)

// UtxoChainSpendView detects on-chain spends using the UTXO cache when synced to the header tip,
// with a raw-block scan fallback (same as ChainSpendView).
type UtxoChainSpendView struct {
	Utxo     UtxoOutpointSource
	Journal  HeaderChain
	Index    TxIndexer
	Raw      RawBlockGetter
	Fallback *ChainSpendView
}

// NewUtxoChainSpendView builds a spend checker; utxo may be nil (scan-only).
func NewUtxoChainSpendView(utxo UtxoOutpointSource, journal HeaderChain, raw RawBlockGetter, index TxIndexer) *UtxoChainSpendView {
	v := &UtxoChainSpendView{
		Utxo:    utxo,
		Journal: journal,
		Index:   index,
		Raw:     raw,
	}
	if journal != nil && raw != nil && index != nil {
		v.Fallback = NewChainSpendView(journal, raw, index)
	}
	return v
}

// OutpointSpent implements on-chain double-spend detection for mempool admission.
func (c *UtxoChainSpendView) OutpointSpent(prevHash [32]byte, vout uint32) (bool, error) {
	if c == nil {
		return false, nil
	}
	if c.Utxo != nil && c.Journal != nil {
		tip, err := c.Journal.TipHeight()
		if err == nil && c.utxoSyncedToTip(tip) {
			if _, _, ok := c.Utxo.UnspentOutpoint(prevHash, vout); ok {
				return false, nil
			}
			spent, err := c.confirmedSpentInUtxoSet(prevHash, vout)
			if err != nil {
				return false, err
			}
			if spent {
				return true, nil
			}
			return false, nil
		}
	}
	if c.Fallback != nil {
		return c.Fallback.OutpointSpent(prevHash, vout)
	}
	return false, nil
}

func (c *UtxoChainSpendView) utxoSyncedToTip(headerTip int64) bool {
	if c.Utxo == nil {
		return false
	}
	type tipper interface {
		TipHeight() int64
	}
	if t, ok := c.Utxo.(tipper); ok {
		return t.TipHeight() >= headerTip
	}
	return false
}

// confirmedSpentInUtxoSet is true when a confirmed output existed but is absent from the tip UTXO set.
func (c *UtxoChainSpendView) confirmedSpentInUtxoSet(prevHash [32]byte, vout uint32) (bool, error) {
	if c.Index == nil || c.Raw == nil {
		return false, nil
	}
	txid := txidDisplayFromLE(prevHash)
	if stx, ok := c.Index.(*store.TxIndex); ok && stx != nil {
		_, _, found := store.LoadIndexedTxVout(stx, c.Raw, txid, vout)
		return found, nil
	}
	blockHash, pos, err := c.Index.Lookup(txid)
	if err != nil {
		return false, nil
	}
	raw, err := c.Raw.Get(blockHash)
	if err != nil {
		return false, fmt.Errorf("spend check: block payload: %w", err)
	}
	tx, _, err := wire.ReadTxAtIndex(raw, pos)
	if err != nil {
		return false, nil
	}
	if int(vout) >= len(tx.Vout) {
		return false, nil
	}
	return true, nil
}
