// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"fmt"

	"dogego/mempool"
	"dogego/wire"
)

// ErrOrphanTx is returned when a transaction is held in the orphan pool pending parents.
var ErrOrphanTx = errors.New("consensus: orphan transaction (missing parent)")

// OrphanStore is the subset of mempool.OrphanPool used for admission.
type OrphanStore interface {
	Add(raw []byte, parentTxIDs []string, fromPeer string) (string, error)
	Remove(displayTxid string)
	ChildrenOf(parentTxid string) [][]byte
}

// MissingParentTxIDs lists RPC display txids for inputs not found in view (non-null prevouts only).
func MissingParentTxIDs(tx *wire.Tx, view PrevOutView) []string {
	if tx == nil || view == nil {
		return nil
	}
	var out []string
	seen := make(map[string]struct{})
	for i := range tx.Vin {
		in := &tx.Vin[i]
		if IsNullOutpoint(in) {
			continue
		}
		if _, ok := view.Lookup(in.PrevHash, in.PrevIdx); ok {
			continue
		}
		id := txidDisplayFromLE(in.PrevHash)
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// AcceptMempoolTxWithOrphans runs admission; on missing prevout stores in orphans and returns ErrOrphanTx.
// After a successful pool add, promotes orphan descendants that become valid.
func AcceptMempoolTxWithOrphans(raw []byte, tx *wire.Tx, pool *mempool.Pool, orphans OrphanStore, adm MempoolAdmission, fromPeer string) error {
	if tx == nil {
		return fmt.Errorf("consensus: nil transaction")
	}
	err := AcceptMempoolTxAdmission(tx, adm)
	if err != nil {
		if errors.Is(err, ErrMissingPrevout) && orphans != nil {
			parents := MissingParentTxIDs(tx, adm.View)
			if len(parents) == 0 {
				return err
			}
			if _, oerr := orphans.Add(raw, parents, fromPeer); oerr != nil {
				return fmt.Errorf("%w (%v)", err, oerr)
			}
			return ErrOrphanTx
		}
		return err
	}
	fees, sizes := MempoolEvictionMaps(pool, adm.View)
	AddCandidateEvictionEntry(tx, raw, adm.View, fees, sizes)
	if err := pool.AddWithEviction(raw, fees, sizes); err != nil {
		return err
	}
	promoteOrphanChildren(pool, orphans, adm, txidDisplayFromLE(tx.TxHash()))
	return nil
}

func promoteOrphanChildren(pool *mempool.Pool, orphans OrphanStore, adm MempoolAdmission, parentTxid string) {
	if orphans == nil || pool == nil {
		return
	}
	queue := orphans.ChildrenOf(parentTxid)
	for len(queue) > 0 {
		raw := queue[0]
		queue = queue[1:]
		child, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		cid := txidDisplayFromLE(child.TxHash())
		if err := AcceptMempoolTxAdmission(child, adm); err != nil {
			if errors.Is(err, ErrMissingPrevout) {
				_, _ = orphans.Add(raw, MissingParentTxIDs(child, adm.View), "")
			}
			continue
		}
		fees, sizes := MempoolEvictionMaps(pool, adm.View)
		AddCandidateEvictionEntry(child, raw, adm.View, fees, sizes)
		if err := pool.AddWithEviction(raw, fees, sizes); err != nil {
			continue
		}
		orphans.Remove(cid)
		queue = append(queue, orphans.ChildrenOf(cid)...)
	}
}
