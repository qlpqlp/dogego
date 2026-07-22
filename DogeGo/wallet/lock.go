// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"fmt"
	"strings"
)

// LockedOutpoint is a txid/vout pair temporarily excluded from coin selection.
type LockedOutpoint struct {
	TxID string
	Vout uint32
}

type lockedOutJSON struct {
	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`
}

func normTxID(txid string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(txid, "0x")))
}

func (w *Disk) loadLockedOutpoints(entries []lockedOutJSON) {
	w.locked = w.locked[:0]
	for _, e := range entries {
		id := normTxID(e.TxID)
		if id == "" {
			continue
		}
		w.locked = append(w.locked, LockedOutpoint{TxID: id, Vout: e.Vout})
	}
}

// ListLockedOutpoints returns a copy of locked outpoints.
func (w *Disk) ListLockedOutpoints() []LockedOutpoint {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]LockedOutpoint, len(w.locked))
	copy(out, w.locked)
	return out
}

// IsLockedOutpoint reports whether an outpoint is locked.
func (w *Disk) IsLockedOutpoint(txid string, vout uint32) bool {
	id := normTxID(txid)
	if id == "" {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, o := range w.locked {
		if o.Vout == vout && o.TxID == id {
			return true
		}
	}
	return false
}

// SetLockedOutpoints replaces the lock set (lockunspent unlock=false) or removes entries (unlock=true).
func (w *Disk) SetLockedOutpoints(unlock bool, outs []LockedOutpoint) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if unlock {
		if len(outs) == 0 {
			w.locked = w.locked[:0]
		} else {
			for _, rem := range outs {
				id := normTxID(rem.TxID)
				var kept []LockedOutpoint
				for _, o := range w.locked {
					if o.TxID == id && o.Vout == rem.Vout {
						continue
					}
					kept = append(kept, o)
				}
				w.locked = kept
			}
		}
	} else {
		seen := make(map[string]struct{})
		for _, o := range w.locked {
			seen[o.TxID+fmt.Sprintf(":%d", o.Vout)] = struct{}{}
		}
		for _, add := range outs {
			id := normTxID(add.TxID)
			if id == "" {
				continue
			}
			key := id + fmt.Sprintf(":%d", add.Vout)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			w.locked = append(w.locked, LockedOutpoint{TxID: id, Vout: add.Vout})
		}
	}
	return w.saveLocked()
}
