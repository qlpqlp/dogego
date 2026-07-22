// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"fmt"
	"sort"
	"strings"
)

// RecordTxReplacement stores a bumpfee replacement (old display txid -> new).
func (w *Disk) RecordTxReplacement(oldTxid, newTxid string) error {
	oldTxid = normalizeWalletTxID(oldTxid)
	newTxid = normalizeWalletTxID(newTxid)
	if len(oldTxid) != 64 || len(newTxid) != 64 || oldTxid == newTxid {
		return fmt.Errorf("invalid replacement txids")
	}
	w.mu.Lock()
	if w.replacements == nil {
		w.replacements = make(map[string]string)
	}
	w.replacements[oldTxid] = newTxid
	w.mu.Unlock()
	return w.saveLocked()
}

// ConflictsForTx returns related txids from persisted bumpfee replacements.
func (w *Disk) ConflictsForTx(txid string) []string {
	txid = normalizeWalletTxID(txid)
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []string
	for old, new := range w.replacements {
		if old == txid {
			out = append(out, new)
		} else if new == txid {
			out = append(out, old)
		}
	}
	sort.Strings(out)
	return out
}

func normalizeWalletTxID(txid string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(txid, "0x")))
}

// RemoveReplacementsForTx deletes bumpfee replacement entries involving txid.
func (w *Disk) RemoveReplacementsForTx(txid string) error {
	txid = normalizeWalletTxID(txid)
	w.mu.Lock()
	changed := false
	for old, new := range w.replacements {
		if old == txid || new == txid {
			delete(w.replacements, old)
			changed = true
		}
	}
	w.mu.Unlock()
	if !changed {
		return nil
	}
	return w.saveLocked()
}
