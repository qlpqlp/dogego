// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"dogego/wire"
)

type blockTxIter func(fn func(uint32, *wire.Tx) error) error

// RemoveForBlock drops mempool transactions included in the block and any that double-spend
// the same inputs (including descendant clusters). Returns removed display txids.
func (p *Pool) RemoveForBlock(pb *wire.ParsedBlock) []string {
	if p == nil || pb == nil {
		return nil
	}
	return p.pruneBlockTxs(func(fn func(uint32, *wire.Tx) error) error {
		for i, tx := range pb.Txs {
			if err := fn(uint32(i), tx); err != nil {
				return err
			}
		}
		return nil
	})
}

// RemoveForBlockRaw is like RemoveForBlock but scans a serialized block without retaining all txs.
func (p *Pool) RemoveForBlockRaw(raw []byte) []string {
	if p == nil || len(raw) < 80 {
		return nil
	}
	return p.pruneBlockTxs(func(fn func(uint32, *wire.Tx) error) error {
		return wire.ForEachBlockTx(raw, fn)
	})
}

func (p *Pool) pruneBlockTxs(iter blockTxIter) []string {
	if p == nil {
		return nil
	}
	p.NoteBlockFound()
	var removed []string
	seen := make(map[string]struct{})

	_ = iter(func(i uint32, tx *wire.Tx) error {
		if i == 0 || isCoinbaseVin(tx) {
			return nil
		}
		id := txidDisplayHex(tx.TxHash())
		if p.RemoveByTxID(id) {
			removed = append(removed, id)
		}
		seen[id] = struct{}{}
		return nil
	})

	_ = iter(func(i uint32, tx *wire.Tx) error {
		if i == 0 || isCoinbaseVin(tx) {
			return nil
		}
		id := txidDisplayHex(tx.TxHash())
		for vout := range tx.Vout {
			spender := p.SpenderOfOutpoint(id, uint32(vout))
			if spender == "" {
				continue
			}
			if _, dup := seen[spender]; dup {
				continue
			}
			cluster, err := p.RemoveCluster(spender)
			if err != nil {
				continue
			}
			for _, cid := range cluster {
				seen[cid] = struct{}{}
				removed = append(removed, cid)
			}
		}
		return nil
	})

	_ = iter(func(i uint32, tx *wire.Tx) error {
		if i == 0 || isCoinbaseVin(tx) {
			return nil
		}
		for _, in := range tx.Vin {
			if isNullOutpoint(in) {
				continue
			}
			spender := p.SpenderOfOutpoint(txidDisplayHex(in.PrevHash), in.PrevIdx)
			if spender == "" {
				continue
			}
			if _, dup := seen[spender]; dup {
				continue
			}
			cluster, err := p.RemoveCluster(spender)
			if err != nil {
				continue
			}
			for _, cid := range cluster {
				seen[cid] = struct{}{}
				removed = append(removed, cid)
			}
		}
		return nil
	})
	return removed
}

func isCoinbaseVin(tx *wire.Tx) bool {
	return tx != nil && len(tx.Vin) == 1 && isNullOutpoint(tx.Vin[0])
}
