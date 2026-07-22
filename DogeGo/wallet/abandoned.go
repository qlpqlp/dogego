// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"fmt"
	"time"
)

// AbandonedTx is a wallet transaction removed from the mempool via abandontransaction.
type AbandonedTx struct {
	TxID        string
	Category    string
	AmountKoinu int64
	Address     string
	Time        int64
}

type abandonedTxJSON struct {
	TxID        string `json:"txid"`
	Category    string `json:"category"`
	AmountKoinu int64  `json:"amount_koinu"`
	Address     string `json:"address,omitempty"`
	Time        int64  `json:"time,omitempty"`
}

func (w *Disk) loadAbandonedTxs(rows []abandonedTxJSON) {
	w.abandoned = w.abandoned[:0]
	for _, r := range rows {
		id := normalizeWalletTxID(r.TxID)
		if len(id) != 64 || r.Category == "" {
			continue
		}
		w.abandoned = append(w.abandoned, AbandonedTx{
			TxID: id, Category: r.Category, AmountKoinu: r.AmountKoinu,
			Address: r.Address, Time: r.Time,
		})
	}
}

// AbandonTx records a transaction abandoned by the user (Core wallet map).
func (w *Disk) AbandonTx(tx AbandonedTx) error {
	tx.TxID = normalizeWalletTxID(tx.TxID)
	if len(tx.TxID) != 64 {
		return fmt.Errorf("invalid txid")
	}
	if tx.Category != "send" && tx.Category != "receive" {
		return fmt.Errorf("invalid category")
	}
	if tx.Time <= 0 {
		tx.Time = time.Now().Unix()
	}
	w.mu.Lock()
	for i, existing := range w.abandoned {
		if existing.TxID == tx.TxID && existing.Category == tx.Category && existing.Address == tx.Address {
			w.abandoned[i] = tx
			w.mu.Unlock()
			return w.saveLocked()
		}
	}
	w.abandoned = append(w.abandoned, tx)
	w.mu.Unlock()
	return w.saveLocked()
}

// IsAbandoned reports whether txid is in the abandoned list.
func (w *Disk) IsAbandoned(txid string) bool {
	txid = normalizeWalletTxID(txid)
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, a := range w.abandoned {
		if a.TxID == txid {
			return true
		}
	}
	return false
}

// ListAbandoned returns a copy of persisted abandoned transactions.
func (w *Disk) ListAbandoned() []AbandonedTx {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]AbandonedTx, len(w.abandoned))
	copy(out, w.abandoned)
	return out
}

// RemoveAbandoned drops txid from the abandoned list (removeprunedfunds).
func (w *Disk) RemoveAbandoned(txid string) bool {
	txid = normalizeWalletTxID(txid)
	w.mu.Lock()
	defer w.mu.Unlock()
	for i, a := range w.abandoned {
		if a.TxID == txid {
			w.abandoned = append(w.abandoned[:i], w.abandoned[i+1:]...)
			_ = w.saveLocked()
			return true
		}
	}
	return false
}
